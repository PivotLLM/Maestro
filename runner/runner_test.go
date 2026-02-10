/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"os"
	"path/filepath"
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

// setupTestRunner creates a test runner with minimal dependencies
func setupTestRunner(t *testing.T) (*testRunner, string) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "maestro-runner-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create subdirectories
	projectsDir := filepath.Join(tmpDir, "projects")
	playbooksDir := filepath.Join(tmpDir, "playbooks")
	if err := os.MkdirAll(projectsDir, 0755); err != nil {
		t.Fatalf("Failed to create projects dir: %v", err)
	}
	if err := os.MkdirAll(playbooksDir, 0755); err != nil {
		t.Fatalf("Failed to create playbooks dir: %v", err)
	}

	// Create config directly (similar to how tests in config_test.go work)
	cfg := &config.Config{}
	// We need to simulate a loaded config - set the internal data and derived paths
	// Use reflection or create a test helper that exposes this, or just create a minimal config file

	// For simplicity, create a minimal config file and load it
	configPath := filepath.Join(tmpDir, "config.json")
	configData := []byte(`{
		"version": 1,
		"base_dir": "` + tmpDir + `",
		"projects_dir": "projects",
		"playbooks_dir": "playbooks",
		"default_llm": "test-llm",
		"llms": [
			{
				"id": "test-llm",
				"display_name": "Test LLM",
				"type": "command",
				"command": "/bin/echo",
				"args": ["{{PROMPT}}"],
				"description": "Test LLM for testing",
				"enabled": true
			}
		],
		"runner": {
			"max_concurrent": 2,
			"max_attempts": 3,
			"retry_delay_seconds": 1,
			"rate_limit_requests": 10,
			"rate_limit_period": 60
		}
	}`)
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg = config.New(config.WithConfigPath(configPath))
	if err := cfg.Load(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Create logger with temp log file
	logPath := filepath.Join(tmpDir, "test.log")
	logger, err := logging.New(logPath)
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Convert config reference dirs to reference service format
	var externalDirs []reference.ExternalDir
	for _, refDir := range cfg.ReferenceDirs() {
		externalDirs = append(externalDirs, reference.ExternalDir{
			Path:  refDir.Path,
			Mount: refDir.Mount,
		})
	}

	// Create services (using the same pattern as in server/server.go)
	referenceService := reference.NewService(
		reference.WithEmbeddedFS(cfg.EmbeddedFS()),
		reference.WithExternalDirs(externalDirs),
		reference.WithLogger(logger),
	)
	playbooksService := playbooks.NewService(cfg.PlaybooksDir(), logger)
	projectsService := projects.NewService(cfg, logger)
	tasksService := tasks.NewService(cfg, projectsService, logger)
	llmService := llm.NewService(cfg, logger, nil)

	// Create runner
	runner := New(cfg, logger, nil, playbooksService, referenceService, llmService, tasksService, projectsService)

	// Store services in runner for test access (or use helper type)
	// For simplicity, we'll just use the runner's internal tasks service via reflection
	// But we need projectsService for creating projects - let's return it too
	return &testRunner{
		Runner:   runner,
		projects: projectsService,
		tasks:    tasksService,
	}, tmpDir
}

// testRunner wraps Runner with test helper services
type testRunner struct {
	*Runner
	projects *projects.Service
	tasks    *tasks.Service
}

// createTestTemplates creates mock template files in the temp directory and returns the templates config
func createTestTemplates(t *testing.T, tmpDir string) *global.DefaultTemplates {
	t.Helper()

	// Create templates directory structure
	templatesDir := filepath.Join(tmpDir, "playbooks", "test", "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates dir: %v", err)
	}

	// Create worker response schema
	workerResponseSchema := `{"type": "object", "properties": {"result": {"type": "string"}}}`
	workerResponsePath := filepath.Join(templatesDir, "worker-response.json")
	if err := os.WriteFile(workerResponsePath, []byte(workerResponseSchema), 0644); err != nil {
		t.Fatalf("Failed to write worker response schema: %v", err)
	}

	// Create worker report template
	workerReportTemplate := `## Worker Report\n{{.WorkResult}}`
	workerReportPath := filepath.Join(templatesDir, "worker-report.md")
	if err := os.WriteFile(workerReportPath, []byte(workerReportTemplate), 0644); err != nil {
		t.Fatalf("Failed to write worker report template: %v", err)
	}

	return &global.DefaultTemplates{
		WorkerResponseTemplate: "test/templates/worker-response.json",
		WorkerReportTemplate:   "test/templates/worker-report.md",
	}
}

func TestGetTaskStatus(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for status testing", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a task set
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", nil, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Create tasks with different statuses
	taskDefs := []struct {
		title  string
		status string
	}{
		{"Task 1", global.ExecutionStatusWaiting},
		{"Task 2", global.ExecutionStatusWaiting},
		{"Task 3", global.ExecutionStatusProcessing},
		{"Task 4", global.ExecutionStatusDone},
		{"Task 5", global.ExecutionStatusFailed},
	}

	taskUUIDs := []string{}
	for _, taskDef := range taskDefs {
		work := &global.WorkExecution{
			Prompt:     "test prompt",
			LLMModelID: "test-llm",
		}
		task, err := runner.tasks.CreateTask(projectName, "main", taskDef.title, "test", work, nil)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
		taskUUIDs = append(taskUUIDs, task.UUID)
	}

	// Update task statuses
	for i, taskDef := range taskDefs {
		updates := map[string]interface{}{
			"work": map[string]interface{}{
				"status": taskDef.status,
			},
		}
		_, err := runner.tasks.UpdateTask(projectName, taskUUIDs[i], updates)
		if err != nil {
			t.Fatalf("Failed to update task %d status: %v", i+1, err)
		}
	}

	// Get task status
	status, err := runner.GetTaskStatus(projectName, "", "")
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}

	// Verify counts
	if status.TotalTasks != 5 {
		t.Errorf("TotalTasks = %d, want 5", status.TotalTasks)
	}
	if status.Pending != 2 {
		t.Errorf("Pending = %d, want 2", status.Pending)
	}
	if status.InProgress != 1 {
		t.Errorf("InProgress = %d, want 1", status.InProgress)
	}
	if status.Done != 1 {
		t.Errorf("Done = %d, want 1", status.Done)
	}
	if status.Failed != 1 {
		t.Errorf("Failed = %d, want 1", status.Failed)
	}
	if status.RunInProgress {
		t.Error("RunInProgress = true, want false (no run started)")
	}
}

func TestGetTaskStatusWithTypeFilter(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for type filtering", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a task set
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", nil, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Create tasks with different types
	taskDefs := []struct {
		title    string
		taskType string
		status   string
	}{
		{"Task 1", "type-a", global.ExecutionStatusWaiting},
		{"Task 2", "type-a", global.ExecutionStatusDone},
		{"Task 3", "type-b", global.ExecutionStatusWaiting},
		{"Task 4", "type-b", global.ExecutionStatusDone},
	}

	taskUUIDs := []string{}
	for _, taskDef := range taskDefs {
		work := &global.WorkExecution{
			Prompt:     "test prompt",
			LLMModelID: "test-llm",
		}
		task, err := runner.tasks.CreateTask(projectName, "main", taskDef.title, taskDef.taskType, work, nil)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}
		taskUUIDs = append(taskUUIDs, task.UUID)
	}

	// Update task statuses
	for i, taskDef := range taskDefs {
		updates := map[string]interface{}{
			"work": map[string]interface{}{
				"status": taskDef.status,
			},
		}
		_, err := runner.tasks.UpdateTask(projectName, taskUUIDs[i], updates)
		if err != nil {
			t.Fatalf("Failed to update task status: %v", err)
		}
	}

	// Get task status filtered by type-a
	status, err := runner.GetTaskStatus(projectName, "", "type-a")
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}

	// Verify counts - should only include type-a tasks
	if status.TotalTasks != 2 {
		t.Errorf("TotalTasks = %d, want 2", status.TotalTasks)
	}
	if status.Pending != 1 {
		t.Errorf("Pending = %d, want 1", status.Pending)
	}
	if status.Done != 1 {
		t.Errorf("Done = %d, want 1", status.Done)
	}
}

func TestRunReturnsImmediately(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for async run", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create test templates (required for runner validation)
	templates := createTestTemplates(t, tmpDir)

	// Create a task set with templates
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", templates, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Create a task that would take time to execute
	// (In reality this won't execute since we don't have a real LLM, but the test
	// verifies that Run() returns immediately regardless)
	work := &global.WorkExecution{
		Prompt:     "test prompt",
		LLMModelID: "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Long Task", "test", work, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Measure time to call Run
	start := time.Now()
	result, err := runner.Run(context.Background(), &global.RunRequest{
		Project: projectName,
		Timeout: 30,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Run should return in less than 1 second (it should be nearly instant)
	if elapsed > time.Second {
		t.Errorf("Run took %v, expected < 1s (should return immediately)", elapsed)
	}

	// Verify result indicates tasks were queued
	if result.TasksFound != 1 {
		t.Errorf("TasksFound = %d, want 1", result.TasksFound)
	}

	// Verify message indicates async execution
	if result.Message == "" {
		t.Error("Expected message about async execution, got empty string")
	}
}

func TestRunConcurrencyPrevention(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for concurrency", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create test templates (required for runner validation)
	templates := createTestTemplates(t, tmpDir)

	// Create a task set with templates
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", templates, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Create a task
	work := &global.WorkExecution{
		Prompt:     "test prompt",
		LLMModelID: "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task", "test", work, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Start first run
	result1, err := runner.Run(context.Background(), &global.RunRequest{
		Project: projectName,
		Timeout: 30,
	})
	if err != nil {
		t.Fatalf("First Run failed: %v", err)
	}

	// Verify first run queued the task
	if result1.TasksFound != 1 {
		t.Errorf("First run TasksFound = %d, want 1", result1.TasksFound)
	}

	// Immediately try to start second run (before first completes)
	result2, err := runner.Run(context.Background(), &global.RunRequest{
		Project: projectName,
		Timeout: 30,
	})
	if err != nil {
		t.Fatalf("Second Run failed: %v", err)
	}

	// Second run should indicate a run is already in progress
	if result2.Message == "" || result2.TasksFound != 0 {
		t.Error("Expected second run to indicate run already in progress")
	}

	// Wait briefly for background goroutine to complete
	time.Sleep(100 * time.Millisecond)
}

func TestGetTaskStatusShowsRunInProgress(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for run tracking", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create test templates (required for runner validation)
	templates := createTestTemplates(t, tmpDir)

	// Create a task set with templates
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", templates, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Create a task
	work := &global.WorkExecution{
		Prompt:     "test prompt",
		LLMModelID: "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task", "test", work, nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Check status before run - should not be in progress
	statusBefore, err := runner.GetTaskStatus(projectName, "", "")
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}
	if statusBefore.RunInProgress {
		t.Error("RunInProgress = true before run started, want false")
	}

	// Start run
	_, err = runner.Run(context.Background(), &global.RunRequest{
		Project: projectName,
		Timeout: 30,
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check status during run - should be in progress
	statusDuring, err := runner.GetTaskStatus(projectName, "", "")
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}
	if !statusDuring.RunInProgress {
		t.Error("RunInProgress = false during run, want true")
	}

	// Wait for background execution to complete
	// (will fail quickly since we don't have a real LLM configured)
	time.Sleep(500 * time.Millisecond)

	// Check status after run - should not be in progress
	statusAfter, err := runner.GetTaskStatus(projectName, "", "")
	if err != nil {
		t.Fatalf("GetTaskStatus failed: %v", err)
	}
	if statusAfter.RunInProgress {
		t.Error("RunInProgress = true after run completed, want false")
	}
}

func TestCreateTaskRequiresPromptField(t *testing.T) {
	runner, tmpDir := setupTestRunner(t)
	defer os.RemoveAll(tmpDir)

	projectName := "test-project"

	// Create a project
	_, err := runner.projects.Create(projectName, "Test Project", "Test project for prompt validation", "", "", "none")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// Create a task set
	_, err = runner.tasks.CreateTaskSet(projectName, "main", "Main Tasks", "Test task set", nil, false, global.Limits{})
	if err != nil {
		t.Fatalf("Failed to create task set: %v", err)
	}

	// Try to create a task with no prompt fields - should fail
	workEmpty := &global.WorkExecution{
		LLMModelID: "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task Without Prompt", "test", workEmpty, nil)
	if err == nil {
		t.Error("Expected error when creating task without prompt fields, got nil")
	} else if err.Error() != "at least one prompt field is required: instructions_file, instructions_text, or prompt" {
		t.Errorf("Got unexpected error: %v", err)
	}

	// Verify task with instructions_file is accepted
	workFile := &global.WorkExecution{
		InstructionsFile:       "instructions.md",
		InstructionsFileSource: "project",
		LLMModelID:             "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task With File", "test", workFile, nil)
	if err != nil {
		t.Errorf("Expected task with instructions_file to succeed, got error: %v", err)
	}

	// Verify task with instructions_text is accepted
	workText := &global.WorkExecution{
		InstructionsText: "inline instructions",
		LLMModelID:       "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task With Text", "test", workText, nil)
	if err != nil {
		t.Errorf("Expected task with instructions_text to succeed, got error: %v", err)
	}

	// Verify task with prompt is accepted
	workPrompt := &global.WorkExecution{
		Prompt:     "task prompt",
		LLMModelID: "test-llm",
	}
	_, err = runner.tasks.CreateTask(projectName, "main", "Task With Prompt", "test", workPrompt, nil)
	if err != nil {
		t.Errorf("Expected task with prompt to succeed, got error: %v", err)
	}
}

// NOTE: Crash recovery tests have been removed during the task set refactoring.
// They tested the old project-based task system which no longer exists.
// New crash recovery tests should be added when the runner is updated to use task sets.
