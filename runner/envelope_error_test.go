/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
)

// TestDispatchErrorMessage_DistinguishesExitCodeAndEnvelope verifies that the
// error string surfaced by the runner clearly distinguishes envelope-only
// failures from exit-code failures.
func TestDispatchErrorMessage_DistinguishesExitCodeAndEnvelope(t *testing.T) {
	cases := []struct {
		name           string
		result         *llm.DispatchResult
		wantContains   string
		notWantContain string
	}{
		{
			name:           "nil result",
			result:         nil,
			wantContains:   "nil result",
			notWantContain: "envelope",
		},
		{
			name: "exit code failure preserves wording",
			result: &llm.DispatchResult{
				ExitCode: 2,
				Stderr:   "boom",
			},
			wantContains:   "LLM exited with code 2",
			notWantContain: "envelope",
		},
		{
			name: "envelope failure flagged distinctly",
			result: &llm.DispatchResult{
				ExitCode:   0,
				IsError:    true,
				StopReason: "turn.failed",
			},
			wantContains:   "LLM reported error envelope",
			notWantContain: "exited with code",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := dispatchErrorMessage(c.result)
			if !strings.Contains(got, c.wantContains) {
				t.Errorf("dispatchErrorMessage = %q, want substring %q", got, c.wantContains)
			}
			if c.notWantContain != "" && strings.Contains(got, c.notWantContain) {
				t.Errorf("dispatchErrorMessage = %q, did not want substring %q", got, c.notWantContain)
			}
		})
	}
}

// TestTruncateForLog_TruncatesLongStrings verifies the LLM-finish error
// truncation helper.
func TestTruncateForLog_TruncatesLongStrings(t *testing.T) {
	long := strings.Repeat("x", 600)
	got := truncateForLog(long, llmFinishErrorMaxLen)
	if len(got) <= llmFinishErrorMaxLen {
		t.Errorf("expected truncation suffix; len(got)=%d", len(got))
	}
	if !strings.HasSuffix(got, "(truncated)") {
		t.Errorf("expected truncated marker; got tail %q", got[len(got)-20:])
	}
	short := "short"
	if truncateForLog(short, llmFinishErrorMaxLen) != short {
		t.Errorf("short string should round-trip unchanged")
	}
}

// envelopeRunnerCase covers the two error-envelope shapes the spec calls out.
type envelopeRunnerCase struct {
	name          string
	outputFormat  string
	envelopeJSON  string
	wantStopMatch string
}

// TestWorkerExecution_ErrorEnvelopeGate spins up a runner whose worker LLM
// emits a parser-detectable error envelope but exits 0. The new gate must
// route this through the retry path (work.Status="retry") rather than the
// schema-validation path.
func TestWorkerExecution_ErrorEnvelopeGate(t *testing.T) {
	cases := []envelopeRunnerCase{
		{
			name:          "claude is_error envelope",
			outputFormat:  "claude",
			envelopeJSON:  `{"type":"result","subtype":"error_during_execution","is_error":true,"result":"","duration_ms":100,"num_turns":1,"usage":{"input_tokens":1,"output_tokens":0},"modelUsage":{"claude-haiku-4-5-20251001":{}}}`,
			wantStopMatch: "error_during_execution",
		},
		{
			name:         "codex turn.failed envelope",
			outputFormat: "codex",
			envelopeJSON: `{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"turn.failed","error":{"message":"rate_limit_exceeded"}}`,
			wantStopMatch: "turn.failed",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runEnvelopeGateCase(t, c)
		})
	}
}

func runEnvelopeGateCase(t *testing.T, c envelopeRunnerCase) {
	t.Helper()

	// Write a shell script that prints the envelope payload regardless of
	// input. We use stdin:true on the LLM so the {{PROMPT}} validation in
	// config doesn't reject our args, and we make the script discard stdin
	// so the runner blocking on stdin doesn't hang.
	scriptDir, err := os.MkdirTemp("", "envelope-llm-*")
	if err != nil {
		t.Fatalf("mkdtemp: %v", err)
	}
	defer os.RemoveAll(scriptDir)
	scriptPath := filepath.Join(scriptDir, "emit.sh")
	script := "#!/bin/sh\ncat >/dev/null\ncat <<'__ENVELOPE_EOF__'\n" + c.envelopeJSON + "\n__ENVELOPE_EOF__\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	llmsJSON, err := json.Marshal(map[string]interface{}{
		"id":            "envelope-llm",
		"type":          "command",
		"command":       scriptPath,
		"args":          []string{},
		"stdin":         true,
		"description":   "envelope-emitting LLM",
		"enabled":       true,
		"output_format": c.outputFormat,
	})
	if err != nil {
		t.Fatalf("marshal llm config: %v", err)
	}
	tr, tmpDir := setupTestRunnerWithLLMConfig(t, string(llmsJSON), "envelope-llm")
	defer os.RemoveAll(tmpDir)

	projectName := "envelope-test"
	if _, err := tr.projects.Create(projectName, "Envelope Test", "envelope gate", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}

	// Templates aren't strictly required for the runner to invoke a worker; the
	// existing TestRunReturnsImmediately path proves this. We still create a
	// minimal taskset.
	templates := createTestTemplates(t, tmpDir)
	if _, err := tr.tasks.CreateTaskSet(projectName, "main", "Main", "envelope gate", templates, false, global.Limits{MaxWorker: 3, MaxRetries: 3, MaxQA: 1}, false, ""); err != nil {
		t.Fatalf("create taskset: %v", err)
	}

	work := &global.WorkExecution{
		Prompt:     "irrelevant",
		LLMModelID: "envelope-llm",
	}
	task, err := tr.tasks.CreateTask(projectName, "main", "envelope-task", "test", work, nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if _, err := tr.Run(context.Background(), &global.RunRequest{Project: projectName}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr.Runner.Wait()

	// Wait briefly for any post-Run goroutine to settle on disk.
	deadline := time.Now().Add(3 * time.Second)
	var finalTask *global.Task
	for time.Now().Before(deadline) {
		ft, _, err := tr.tasks.GetTask(projectName, task.UUID)
		if err == nil && ft != nil {
			finalTask = ft
			if ft.Work.Status != global.ExecutionStatusWaiting && ft.Work.Status != global.ExecutionStatusProcessing {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	if finalTask == nil {
		t.Fatalf("could not load final task")
	}

	// The gate should classify this as a worker failure and either retry or
	// failed — never "done", because the envelope said is_error/turn.failed.
	switch finalTask.Work.Status {
	case global.ExecutionStatusRetry, global.ExecutionStatusFailed:
		// expected — gate fired
	case global.ExecutionStatusDone:
		t.Fatalf("worker status = done; the error envelope should have failed the task")
	default:
		t.Fatalf("worker status = %q; want retry or failed", finalTask.Work.Status)
	}

	if finalTask.Work.Error == "" {
		t.Errorf("expected work.error to be populated")
	}

	// History should record the response with the parsed envelope-error fields.
	historyPath := filepath.Join(tr.tasks.GetResultsDir(projectName), task.UUID+".json")
	if _, err := os.Stat(historyPath); err == nil {
		data, _ := os.ReadFile(historyPath)
		var taskResult global.TaskResult
		if err := json.Unmarshal(data, &taskResult); err == nil {
			foundResp := false
			for _, m := range taskResult.History {
				if m.Type == "response" {
					foundResp = true
					if !m.IsError {
						t.Errorf("history response message: IsError=false, want true")
					}
					if m.Success {
						t.Errorf("history response message: Success=true, want false")
					}
					if c.wantStopMatch != "" && !strings.Contains(m.StopReason, c.wantStopMatch) {
						t.Errorf("history response StopReason = %q, want contains %q", m.StopReason, c.wantStopMatch)
					}
					if m.ResponseSize == 0 {
						t.Errorf("history response ResponseSize = 0, want > 0")
					}
					if m.BytesReceived == 0 {
						t.Errorf("history response BytesReceived = 0, want > 0")
					}
				}
			}
			if !foundResp {
				t.Errorf("history did not contain a response message")
			}
		}
	}
}

// TestRecordHistoryResponse_PopulatesResourceFields verifies the audit-trail
// recording wiring directly: handed a DispatchResult, the history Message
// must carry the new resource accounting fields.
func TestRecordHistoryResponse_PopulatesResourceFields(t *testing.T) {
	tr, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	result := &llm.DispatchResult{
		ExitCode:            0,
		Stdout:              "raw",
		Stderr:              "",
		ResponseSize:        3,
		IsError:             false,
		NormalTermination:   true,
		StopReason:          "end_turn",
		NumTurns:            1,
		InputTokens:         10,
		OutputTokens:        74,
		CacheReadTokens:     0,
		CacheCreationTokens: 85968,
		CostUSD:             0.10784,
		DurationMs:          2213,
		BytesSent:           42,
		BytesReceived:       3,
		ProviderModel:       "claude-haiku-4-5-20251001",
		Success:             true,
	}

	tr.Runner.recordHistoryResponse("test-uuid", "worker", result, "test-llm", 1)

	historyAny, _ := tr.Runner.taskHistory.Load("test-uuid")
	if historyAny == nil {
		t.Fatalf("expected history entry, got nil")
	}
	history := historyAny.([]global.Message)
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	m := history[0]

	if m.ProviderModel != "claude-haiku-4-5-20251001" {
		t.Errorf("ProviderModel = %q", m.ProviderModel)
	}
	if m.NumTurns != 1 {
		t.Errorf("NumTurns = %d", m.NumTurns)
	}
	if m.InputTokens != 10 || m.OutputTokens != 74 {
		t.Errorf("tokens = (%d,%d), want (10,74)", m.InputTokens, m.OutputTokens)
	}
	if m.CacheCreationTokens != 85968 || m.CacheReadTokens != 0 {
		t.Errorf("cache tokens = (%d,%d)", m.CacheCreationTokens, m.CacheReadTokens)
	}
	if m.CostUSD < 0.107 || m.CostUSD > 0.109 {
		t.Errorf("CostUSD = %v", m.CostUSD)
	}
	if m.DurationMs != 2213 {
		t.Errorf("DurationMs = %d", m.DurationMs)
	}
	if m.BytesSent != 42 || m.BytesReceived != 3 {
		t.Errorf("BytesSent/BytesReceived = (%d,%d)", m.BytesSent, m.BytesReceived)
	}
	if !m.Success {
		t.Errorf("Success = false, want true")
	}
	if m.StopReason != "end_turn" {
		t.Errorf("StopReason = %q", m.StopReason)
	}

	// JSON round-trip must additively preserve the new fields without breaking
	// existing readers.
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"provider_model":"claude-haiku-4-5-20251001"`) {
		t.Errorf("JSON missing provider_model: %s", raw)
	}
	if !strings.Contains(string(raw), `"cost_usd"`) {
		t.Errorf("JSON missing cost_usd: %s", raw)
	}
	if !strings.Contains(string(raw), `"input_tokens":10`) {
		t.Errorf("JSON missing input_tokens: %s", raw)
	}
}
