/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/playbooks"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/PivotLLM/Maestro/reference"
	"github.com/PivotLLM/Maestro/tasks"
)

// setupTestRunnerWithLLMConfig builds a runner whose config.json contains the
// given raw "llms" array (a JSON snippet without the enclosing brackets).
// Pass an empty string to simulate "no LLMs configured", or a list with
// disabled entries to simulate "no LLMs enabled".
func setupTestRunnerWithLLMConfig(t *testing.T, llmsJSON, defaultLLM string) (*testRunner, string) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "maestro-dispatch-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	projectsDir := filepath.Join(tmpDir, "projects")
	playbooksDir := filepath.Join(tmpDir, "playbooks")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("Failed to create projects dir: %v", err)
	}
	if err := os.MkdirAll(playbooksDir, 0755); err != nil {
		t.Fatalf("Failed to create playbooks dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.json")
	defaultLLMField := ""
	if defaultLLM != "" {
		defaultLLMField = `"default_llm": "` + defaultLLM + `",`
	}
	configData := []byte(`{
		"version": 1,
		"base_dir": "` + tmpDir + `",
		"projects_dir": "projects",
		"playbooks_dir": "playbooks",
		` + defaultLLMField + `
		"llms": [` + llmsJSON + `],
		"runner": {
			"max_concurrent": 2,
			"max_attempts": 3,
			"retry_delay_seconds": 1,
			"rate_limit_requests": 100,
			"rate_limit_period": 60
		}
	}`)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg := config.New(config.WithConfigPath(configPath))
	if err := cfg.Load(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	logPath := filepath.Join(tmpDir, "test.log")
	logger, err := logging.New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	var externalDirs []reference.ExternalDir
	for _, refDir := range cfg.ReferenceDirs() {
		externalDirs = append(externalDirs, reference.ExternalDir{
			Path:  refDir.Path,
			Mount: refDir.Mount,
		})
	}

	referenceService := reference.NewService(
		reference.WithEmbeddedFS(cfg.EmbeddedFS()),
		reference.WithExternalDirs(externalDirs),
		reference.WithLogger(logger),
	)
	playbooksService := playbooks.NewService(cfg.PlaybooksDir(), logger)
	projectsService := projects.NewService(cfg, logger)
	tasksService := tasks.NewService(cfg, projectsService, logger)
	llmService := llm.NewService(cfg, logger, nil)

	runner := New(cfg, logger, nil, playbooksService, referenceService, llmService, tasksService, projectsService)

	return &testRunner{
		Runner:   runner,
		projects: projectsService,
		tasks:    tasksService,
	}, tmpDir
}

// callbackRecorder is a goroutine-safe recorder for CompletionSink deliveries.
type callbackRecorder struct {
	mu       sync.Mutex
	received []CallbackPayload
	rawBody  []string
	done     chan struct{}
	once     sync.Once
}

func newCallbackRecorder() *callbackRecorder {
	return &callbackRecorder{done: make(chan struct{})}
}

// sink is a runner.CompletionSink that records each delivered payload.
func (c *callbackRecorder) sink(body []byte) {
	var payload CallbackPayload
	if err := json.Unmarshal(body, &payload); err == nil {
		c.mu.Lock()
		c.received = append(c.received, payload)
		c.rawBody = append(c.rawBody, string(body))
		c.mu.Unlock()
	}
	c.once.Do(func() { close(c.done) })
}

func (c *callbackRecorder) wait(t *testing.T, d time.Duration) CallbackPayload {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(d):
		t.Fatalf("Timed out waiting for callback after %v", d)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.received) == 0 {
		t.Fatalf("No callback recorded")
	}
	return c.received[0]
}

// TestDispatch_NoLLMsEnabled exercises the bug fix: when all LLMs are disabled,
// dispatch must mark the task failed, write a result file, fire a "failed"
// callback, and clean up the lock file.
func TestDispatch_NoLLMsEnabled(t *testing.T) {
	llmsJSON := `{
		"id": "test-llm",
		"type": "command",
		"command": "/bin/echo",
		"args": ["{{PROMPT}}"],
		"description": "Disabled test LLM",
		"enabled": false
	}`
	runner, tmpDir := setupTestRunnerWithLLMConfig(t, llmsJSON, "")
	defer os.RemoveAll(tmpDir)

	rec := newCallbackRecorder()

	projectName := "test-project"
	if _, err := runner.projects.Create(projectName, "Test Project", "no-llm dispatch test", "", "", "none"); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	req := &DispatchRequest{
		Project: projectName,
		Path:    "dispatch/no-llm",
		Title:   "no-llm dispatch",
		Prompt:  "this prompt should never reach an LLM",
	}

	dispatchResult, err := runner.RunDispatch(req, rec.sink)
	if err != nil {
		t.Fatalf("RunDispatch returned error: %v", err)
	}
	if dispatchResult == nil {
		t.Fatal("RunDispatch returned nil result")
	}

	payload := rec.wait(t, 5*time.Second)
	runner.Wait()

	// Check lock file first — any service call that uses withLock would re-create
	// the file (flock leaves the file behind on Unlock), so this must precede
	// the on-disk verifications below that go through tasks.Service.
	tasksDir := filepath.Join(tmpDir, "projects", projectName, global.TasksDir)
	lockPath := filepath.Join(tasksDir, "dispatch__no-llm.json.lock")
	if _, statErr := os.Stat(lockPath); !os.IsNotExist(statErr) {
		t.Errorf("Lock file still present at %s (leak)", lockPath)
	}

	if payload.Event != callbackEventFailed {
		t.Errorf("payload.Event = %q, want %q", payload.Event, callbackEventFailed)
	}
	if payload.ErrorCode != "no_llm_enabled" {
		t.Errorf("payload.ErrorCode = %q, want %q", payload.ErrorCode, "no_llm_enabled")
	}
	if !strings.Contains(payload.ErrorMessage, "no LLMs are enabled") {
		t.Errorf("payload.ErrorMessage = %q, want it to mention LLMs", payload.ErrorMessage)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("payload.Tasks count = %d, want 1", len(payload.Tasks))
	}
	if payload.Tasks[0].Status != global.ExecutionStatusFailed {
		t.Errorf("payload.Tasks[0].Status = %q, want %q", payload.Tasks[0].Status, global.ExecutionStatusFailed)
	}
	if payload.Tasks[0].ErrorCode != "no_llm_enabled" {
		t.Errorf("payload.Tasks[0].ErrorCode = %q, want %q", payload.Tasks[0].ErrorCode, "no_llm_enabled")
	}

	// Verify a result file was written and reflects the failure.
	taskUUID := dispatchResult.UUID
	resultPath := filepath.Join(runner.tasks.GetResultsDir(projectName), taskUUID+".json")
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("Result file missing for failed dispatch: %v", err)
	}
	var taskResult global.TaskResult
	if err := json.Unmarshal(resultData, &taskResult); err != nil {
		t.Fatalf("Failed to parse result file: %v", err)
	}
	if taskResult.Worker.Status != global.ExecutionStatusFailed {
		t.Errorf("Result file worker.status = %q, want %q", taskResult.Worker.Status, global.ExecutionStatusFailed)
	}
	if taskResult.Worker.ErrorCode != "no_llm_enabled" {
		t.Errorf("Result file worker.error_code = %q, want %q", taskResult.Worker.ErrorCode, "no_llm_enabled")
	}
	if taskResult.CompletedAt.IsZero() {
		t.Errorf("Result file completed_at should be set, got zero value")
	}

	// Reload the taskset (this would re-create the lock file, but the lock
	// assertion above has already run).
	ts, err := runner.tasks.GetTaskSet(projectName, "dispatch/no-llm")
	if err != nil {
		t.Fatalf("Failed to reload taskset: %v", err)
	}
	if len(ts.Tasks) != 1 {
		t.Fatalf("Reloaded taskset.Tasks count = %d, want 1", len(ts.Tasks))
	}
	if ts.Tasks[0].Work.Status != global.ExecutionStatusFailed {
		t.Errorf("On-disk task.Work.Status = %q, want %q", ts.Tasks[0].Work.Status, global.ExecutionStatusFailed)
	}
	if ts.Tasks[0].Work.ErrorCode != "no_llm_enabled" {
		t.Errorf("On-disk task.Work.ErrorCode = %q, want %q", ts.Tasks[0].Work.ErrorCode, "no_llm_enabled")
	}
}

// TestDispatch_BuildPromptFailure exercises a second pre-execution failure path.
// Requesting a project file that does not exist makes buildPrompt fail; the
// dispatch goroutine must coerce the task into terminal failed state.
func TestDispatch_BuildPromptFailure(t *testing.T) {
	llmsJSON := `{
		"id": "test-llm",
		"type": "command",
		"command": "/bin/echo",
		"args": ["{{PROMPT}}"],
		"description": "Enabled test LLM",
		"enabled": true
	}`
	runner, tmpDir := setupTestRunnerWithLLMConfig(t, llmsJSON, "test-llm")
	defer os.RemoveAll(tmpDir)

	rec := newCallbackRecorder()

	projectName := "test-project"
	if _, err := runner.projects.Create(projectName, "Test Project", "buildPrompt failure test", "", "", "none"); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	req := &DispatchRequest{
		Project:                projectName,
		Path:                   "dispatch/missing-file",
		Title:                  "missing-file dispatch",
		Prompt:                 "irrelevant",
		InstructionsFile:       "does-not-exist.txt",
		InstructionsFileSource: "project_files",
	}

	dispatchResult, err := runner.RunDispatch(req, rec.sink)
	if err != nil {
		t.Fatalf("RunDispatch returned error: %v", err)
	}

	payload := rec.wait(t, 5*time.Second)
	runner.Wait()

	// Check lock file first — service-level reads would re-create it.
	tasksDir := filepath.Join(tmpDir, "projects", projectName, global.TasksDir)
	lockPath := filepath.Join(tasksDir, "dispatch__missing-file.json.lock")
	if _, statErr := os.Stat(lockPath); !os.IsNotExist(statErr) {
		t.Errorf("Lock file still present at %s (leak)", lockPath)
	}

	if payload.Event != callbackEventFailed {
		t.Errorf("payload.Event = %q, want %q", payload.Event, callbackEventFailed)
	}
	if payload.ErrorCode == "" {
		t.Errorf("payload.ErrorCode should be set on failure, got empty")
	}
	if payload.ErrorMessage == "" {
		t.Errorf("payload.ErrorMessage should be set on failure, got empty")
	}
	if len(payload.Tasks) != 1 || payload.Tasks[0].Status != global.ExecutionStatusFailed {
		t.Errorf("payload.Tasks status = %v, want one task in %q", payload.Tasks, global.ExecutionStatusFailed)
	}

	// Verify a result file exists.
	resultPath := filepath.Join(runner.tasks.GetResultsDir(projectName), dispatchResult.UUID+".json")
	if _, err := os.Stat(resultPath); err != nil {
		t.Fatalf("Result file missing for failed dispatch: %v", err)
	}

	// Reload the taskset (re-creates lock file, but assertion above has run).
	ts, err := runner.tasks.GetTaskSet(projectName, "dispatch/missing-file")
	if err != nil {
		t.Fatalf("Failed to reload taskset: %v", err)
	}
	if ts.Tasks[0].Work.Status != global.ExecutionStatusFailed {
		t.Errorf("On-disk task.Work.Status = %q, want %q", ts.Tasks[0].Work.Status, global.ExecutionStatusFailed)
	}
}

// TestDispatch_SuccessCallback verifies the success path still works: the
// callback fires with event="completed", no top-level error, and the task is
// in done state. The lock file is also cleaned up.
func TestDispatch_SuccessCallback(t *testing.T) {
	llmsJSON := `{
		"id": "test-llm",
		"type": "command",
		"command": "/bin/echo",
		"args": ["{{PROMPT}}"],
		"description": "Enabled test LLM",
		"enabled": true
	}`
	runner, tmpDir := setupTestRunnerWithLLMConfig(t, llmsJSON, "test-llm")
	defer os.RemoveAll(tmpDir)

	rec := newCallbackRecorder()

	projectName := "test-project"
	if _, err := runner.projects.Create(projectName, "Test Project", "dispatch success test", "", "", "none"); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	req := &DispatchRequest{
		Project: projectName,
		Path:    "dispatch/ok",
		Title:   "ok dispatch",
		Prompt:  "hello",
	}

	if _, err := runner.RunDispatch(req, rec.sink); err != nil {
		t.Fatalf("RunDispatch returned error: %v", err)
	}

	payload := rec.wait(t, 10*time.Second)
	runner.Wait()

	// Check lock file first to avoid the test re-creating it via service calls.
	tasksDir := filepath.Join(tmpDir, "projects", projectName, global.TasksDir)
	lockPath := filepath.Join(tasksDir, "dispatch__ok.json.lock")
	if _, statErr := os.Stat(lockPath); !os.IsNotExist(statErr) {
		t.Errorf("Lock file still present at %s (leak)", lockPath)
	}

	if payload.Event != callbackEventCompleted {
		t.Errorf("payload.Event = %q, want %q", payload.Event, callbackEventCompleted)
	}
	if payload.ErrorCode != "" {
		t.Errorf("payload.ErrorCode should be empty on success, got %q", payload.ErrorCode)
	}
	if payload.ErrorMessage != "" {
		t.Errorf("payload.ErrorMessage should be empty on success, got %q", payload.ErrorMessage)
	}
	if len(payload.Tasks) != 1 {
		t.Fatalf("payload.Tasks count = %d, want 1", len(payload.Tasks))
	}
	if payload.Tasks[0].Status != global.ExecutionStatusDone {
		t.Errorf("payload.Tasks[0].Status = %q, want %q", payload.Tasks[0].Status, global.ExecutionStatusDone)
	}
	if payload.Tasks[0].Error != "" || payload.Tasks[0].ErrorCode != "" {
		t.Errorf("Per-task error fields should be empty on success: %+v", payload.Tasks[0])
	}
}

// TestDispatch_GetTaskFailureAfterCreate exercises the F2 fix: when GetTask
// fails immediately after a successful CreateTask inside the dispatch goroutine,
// the goroutine must mark the task terminally failed (result file + on-disk
// status) and release the lock BEFORE firing the failed callback. The early
// GetTask is injected so the failure is deterministic.
func TestDispatch_GetTaskFailureAfterCreate(t *testing.T) {
	llmsJSON := `{
		"id": "test-llm",
		"type": "command",
		"command": "/bin/echo",
		"args": ["{{PROMPT}}"],
		"description": "Enabled test LLM",
		"enabled": true
	}`
	runner, tmpDir := setupTestRunnerWithLLMConfig(t, llmsJSON, "test-llm")
	defer os.RemoveAll(tmpDir)

	rec := newCallbackRecorder()

	projectName := "test-project"
	if _, err := runner.projects.Create(projectName, "Test Project", "GetTask failure test", "", "", "none"); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create the taskset and task on disk via the real service so the lock file
	// exists and failTaskPreExecution's UpdateTask can mark the on-disk task
	// failed. Only the early GetTask call is mocked.
	path := "dispatch/get-task-fails"
	title := "get-task-fails dispatch"
	if _, err := runner.tasks.CreateTaskSet(projectName, path, title, "", nil, false, global.Limits{}, true, ""); err != nil {
		t.Fatalf("Failed to create taskset: %v", err)
	}
	work := &global.WorkExecution{
		Prompt: "irrelevant",
		Status: global.ExecutionStatusWaiting,
	}
	task, err := runner.tasks.CreateTask(projectName, path, title, "", work, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	req := &DispatchRequest{
		Project: projectName,
		Path:    path,
		Title:   title,
		Prompt:  "irrelevant",
	}

	failingGetTask := func(_, _ string) (*global.Task, string, error) {
		return nil, "", errors.New("simulated GetTask failure")
	}

	runner.activeRuns.Add(1)
	runner.runDispatchExecution(req, task, path, failingGetTask, rec.sink)

	payload := rec.wait(t, 5*time.Second)
	runner.Wait()

	// 1. Lock file released. Verify before any service-level read so we don't
	// re-create it via withLock.
	tasksDir := filepath.Join(tmpDir, "projects", projectName, global.TasksDir)
	lockPath := filepath.Join(tasksDir, "dispatch__get-task-fails.json.lock")
	if _, statErr := os.Stat(lockPath); !os.IsNotExist(statErr) {
		t.Errorf("Lock file still present at %s (leak)", lockPath)
	}

	// 2. Callback fired with event=failed and the F1 error_code key populated.
	if payload.Event != callbackEventFailed {
		t.Errorf("payload.Event = %q, want %q", payload.Event, callbackEventFailed)
	}
	if payload.ErrorCode != "task_load_failed" {
		t.Errorf("payload.ErrorCode = %q, want %q", payload.ErrorCode, "task_load_failed")
	}
	if !strings.Contains(payload.ErrorMessage, "simulated GetTask failure") {
		t.Errorf("payload.ErrorMessage = %q, want it to contain the underlying error", payload.ErrorMessage)
	}

	// 3. Result file written with failed status + error_code.
	resultPath := filepath.Join(runner.tasks.GetResultsDir(projectName), task.UUID+".json")
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("Result file missing for failed dispatch: %v", err)
	}
	var taskResult global.TaskResult
	if err := json.Unmarshal(resultData, &taskResult); err != nil {
		t.Fatalf("Failed to parse result file: %v", err)
	}
	if taskResult.Worker.Status != global.ExecutionStatusFailed {
		t.Errorf("Result file worker.status = %q, want %q", taskResult.Worker.Status, global.ExecutionStatusFailed)
	}
	if taskResult.Worker.ErrorCode != "task_load_failed" {
		t.Errorf("Result file worker.error_code = %q, want %q", taskResult.Worker.ErrorCode, "task_load_failed")
	}

	// 4. On-disk task marked terminally failed.
	ts, err := runner.tasks.GetTaskSet(projectName, path)
	if err != nil {
		t.Fatalf("Failed to reload taskset: %v", err)
	}
	if len(ts.Tasks) != 1 {
		t.Fatalf("Reloaded taskset.Tasks count = %d, want 1", len(ts.Tasks))
	}
	if ts.Tasks[0].Work.Status != global.ExecutionStatusFailed {
		t.Errorf("On-disk task.Work.Status = %q, want %q", ts.Tasks[0].Work.Status, global.ExecutionStatusFailed)
	}
	if ts.Tasks[0].Work.ErrorCode != "task_load_failed" {
		t.Errorf("On-disk task.Work.ErrorCode = %q, want %q", ts.Tasks[0].Work.ErrorCode, "task_load_failed")
	}
}
