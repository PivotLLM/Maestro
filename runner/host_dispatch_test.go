/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
)

// hostCtxKey is a context key a host would use to carry orchestration state
// (e.g. sub-agent depth) from the tool call into the dispatch.
type hostCtxKey struct{}

// recordingDispatcher is a host-style llm.Dispatcher that records the context
// value it was dispatched with and returns a canned result or error.
type recordingDispatcher struct {
	mu       sync.Mutex
	calls    int
	ctxValue any
	ctxErr   error // ctx.Err() observed at dispatch time
	err      error // returned from Dispatch when non-nil
}

func (d *recordingDispatcher) Dispatch(ctx context.Context, _ *llm.DispatchRequest) (*llm.DispatchResult, error) {
	d.mu.Lock()
	d.calls++
	d.ctxValue = ctx.Value(hostCtxKey{})
	d.ctxErr = ctx.Err()
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &llm.DispatchResult{
		ExitCode: 0, Stdout: "ok", Text: "ok",
		ResponseParsed: true, NormalTermination: true, Success: true,
	}, nil
}

func (d *recordingDispatcher) GetLLM(id string) *config.LLM        { return &config.LLM{ID: id} }
func (d *recordingDispatcher) GetExecInfo(string) *llm.LLMExecInfo { return &llm.LLMExecInfo{} }
func (d *recordingDispatcher) TestLLM(string) (bool, error)        { return true, nil }

func (d *recordingDispatcher) snapshot() (int, any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls, d.ctxValue, d.ctxErr
}

// newHostDispatchedRunner builds a runner driven by a host dispatcher (as when
// Maestro is embedded), with one project + task set ready to receive tasks.
func newHostDispatchedRunner(t *testing.T, d *recordingDispatcher) (*testRunner, string, string) {
	t.Helper()
	tr, tmpDir := setupTestRunner(t)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })
	tr.Runner.llm = d
	tr.Runner.SetHostDispatched(true)

	project := "host-project"
	if _, err := tr.projects.Create(project, "Host Project", "host dispatch tests", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := tr.tasks.CreateTaskSet(project, "main", "Main", "", nil, false, global.Limits{}, true, ""); err != nil {
		t.Fatalf("create task set: %v", err)
	}
	return tr, tmpDir, project
}

// TestRun_HostDispatch_KeepsCallerContextValues: Run detaches from the tool
// call's cancellation but must keep its values, so a host dispatcher sees the
// same orchestration state (sub-agent depth) as the call that started the run.
func TestRun_HostDispatch_KeepsCallerContextValues(t *testing.T) {
	d := &recordingDispatcher{}
	tr, _, project := newHostDispatchedRunner(t, d)

	if _, err := tr.tasks.CreateTask(project, "main", "worker", "", &global.WorkExecution{Prompt: "do it"}, nil); err != nil {
		t.Fatalf("create task: %v", err)
	}

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), hostCtxKey{}, "depth=2"))
	if _, err := tr.Run(ctx, &global.RunRequest{Project: project}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The tool call returns before the run finishes; its context ends here.
	cancel()
	tr.Runner.Wait()

	calls, got, ctxErr := d.snapshot()
	if calls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", calls)
	}
	if got != "depth=2" {
		t.Errorf("dispatcher saw ctx value %v, want %q (values dropped at Run boundary)", got, "depth=2")
	}
	if ctxErr != nil {
		t.Errorf("dispatch ctx was cancelled (%v); the run must be detached from the caller's cancellation", ctxErr)
	}
}

// TestRunDispatch_HostDispatch_KeepsCallerContextValues: same contract for the
// single-task dispatch path.
func TestRunDispatch_HostDispatch_KeepsCallerContextValues(t *testing.T) {
	d := &recordingDispatcher{}
	tr, _, project := newHostDispatchedRunner(t, d)

	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), hostCtxKey{}, "depth=1"))
	res, err := tr.RunDispatch(ctx, &DispatchRequest{Project: project, Prompt: "do it"}, nil)
	if err != nil {
		t.Fatalf("RunDispatch: %v", err)
	}
	cancel()
	tr.Runner.Wait()

	calls, got, ctxErr := d.snapshot()
	if calls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", calls)
	}
	if got != "depth=1" {
		t.Errorf("dispatcher saw ctx value %v, want %q (values dropped at RunDispatch boundary)", got, "depth=1")
	}
	if ctxErr != nil {
		t.Errorf("dispatch ctx was cancelled (%v); the dispatch must be detached from the caller's cancellation", ctxErr)
	}

	task, _, err := tr.tasks.GetTask(project, res.UUID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Work.Status != global.ExecutionStatusDone {
		t.Errorf("task status = %q, want done", task.Work.Status)
	}
}

// TestRunDispatch_NilContext: a caller passing a nil context must not panic.
func TestRunDispatch_NilContext(t *testing.T) {
	d := &recordingDispatcher{}
	tr, _, project := newHostDispatchedRunner(t, d)

	//nolint:staticcheck // deliberately nil to exercise the guard
	if _, err := tr.RunDispatch(nil, &DispatchRequest{Project: project, Prompt: "do it"}, nil); err != nil {
		t.Fatalf("RunDispatch: %v", err)
	}
	tr.Runner.Wait()
	if calls, _, _ := d.snapshot(); calls != 1 {
		t.Errorf("dispatch calls = %d, want 1", calls)
	}
}

// TestExecuteTask_PermanentError_NoRetry: a permanent dispatch error fails the
// task on the first attempt and does not consume infrastructure retries.
func TestExecuteTask_PermanentError_NoRetry(t *testing.T) {
	d := &recordingDispatcher{err: llm.Permanent(errors.New("maximum sub-agent depth (3) reached"))}
	tr, _, project := newHostDispatchedRunner(t, d)

	res, err := tr.RunDispatch(context.Background(), &DispatchRequest{Project: project, Prompt: "nest deeper"}, nil)
	if err != nil {
		t.Fatalf("RunDispatch: %v", err)
	}
	tr.Runner.Wait()

	if calls, _, _ := d.snapshot(); calls != 1 {
		t.Errorf("dispatch calls = %d, want exactly 1 (permanent errors must not be retried)", calls)
	}

	task, _, err := tr.tasks.GetTask(project, res.UUID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Work.Status != global.ExecutionStatusFailed {
		t.Errorf("task status = %q, want failed", task.Work.Status)
	}
	if task.Work.InfraRetries != 0 {
		t.Errorf("infra_retries = %d, want 0", task.Work.InfraRetries)
	}
	if !strings.Contains(task.Work.Error, "permanent dispatch error") || !strings.Contains(task.Work.Error, "depth") {
		t.Errorf("task error = %q, want it to name the permanent cause", task.Work.Error)
	}

	data, err := os.ReadFile(filepath.Join(tr.tasks.GetResultsDir(project), res.UUID+".json"))
	if err != nil {
		t.Fatalf("result file: %v", err)
	}
	var tr2 global.TaskResult
	if err := json.Unmarshal(data, &tr2); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if tr2.Worker.ErrorCode != "dispatch_permanent_error" {
		t.Errorf("result error_code = %q, want dispatch_permanent_error", tr2.Worker.ErrorCode)
	}
}

// TestExecuteTask_TransientError_StillRetries: an ordinary dispatch error keeps
// the existing infrastructure-retry behaviour (the task is left retryable).
func TestExecuteTask_TransientError_StillRetries(t *testing.T) {
	d := &recordingDispatcher{err: errors.New("connection reset")}
	tr, _, project := newHostDispatchedRunner(t, d)

	res, err := tr.RunDispatch(context.Background(), &DispatchRequest{Project: project, Prompt: "flaky"}, nil)
	if err != nil {
		t.Fatalf("RunDispatch: %v", err)
	}
	tr.Runner.Wait()

	task, _, err := tr.tasks.GetTask(project, res.UUID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if task.Work.InfraRetries != 1 {
		t.Errorf("infra_retries = %d, want 1 (transient errors consume a retry)", task.Work.InfraRetries)
	}
	if strings.Contains(task.Work.Error, "permanent dispatch error") {
		t.Errorf("transient error was classified as permanent: %q", task.Work.Error)
	}
}
