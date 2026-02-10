/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/library"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/playbooks"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/PivotLLM/Maestro/reference"
	"github.com/PivotLLM/Maestro/reporting"
	"github.com/PivotLLM/Maestro/tasks"
	"github.com/PivotLLM/Maestro/templates"
)

// Runner executes tasks via configured LLMs
type Runner struct {
	config          *config.Config
	logger          *logging.Logger
	library         *library.Service
	playbooks       *playbooks.Service
	reference       *reference.Service
	llm             *llm.Service
	tasks           *tasks.Service
	projects        *projects.Service
	reporter        *reporting.Reporter
	validator       *templates.Validator
	rateLimiter     *RateLimiter
	runningProjects sync.Map       // map[string]bool - tracks which projects have runs in progress
	taskHistory     sync.Map       // map[string][]global.Message - accumulates history by task UUID
	activeRuns      sync.WaitGroup // tracks active run goroutines for graceful shutdown
}

// recoveryState tracks the state of recovery mode during a run.
// Recovery mode is entered when an LLM returns a non-zero exit code or rate limit is detected.
type recoveryState struct {
	inRecovery    bool        // whether we're currently in recovery mode
	enteredAt     time.Time   // when recovery mode was entered
	scheduleIndex int         // current index in test_schedule_seconds
	llmID         string      // which LLM triggered recovery
	llmConfig     *config.LLM // LLM config for rate limit patterns
	mu            sync.Mutex  // protects state updates
}

// newRecoveryState creates a new recovery state tracker
func newRecoveryState() *recoveryState {
	return &recoveryState{}
}

// enterRecovery enters recovery mode for the given LLM
func (rs *recoveryState) enterRecovery(llmID string, llmConfig *config.LLM) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.inRecovery {
		// Already in recovery - just reset the timer by keeping enteredAt as-is
		// but don't advance the schedule
		return
	}

	rs.inRecovery = true
	rs.enteredAt = time.Now()
	rs.scheduleIndex = 0
	rs.llmID = llmID
	rs.llmConfig = llmConfig
}

// resetWaitTimer resets the wait timer when another failure arrives during recovery
func (rs *recoveryState) resetWaitTimer() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	// Reset enteredAt to extend the current wait period
	rs.enteredAt = time.Now()
}

// exitRecovery exits recovery mode
func (rs *recoveryState) exitRecovery() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.inRecovery = false
	rs.scheduleIndex = 0
	rs.llmID = ""
	rs.llmConfig = nil
}

// advanceSchedule moves to the next interval in the test schedule
func (rs *recoveryState) advanceSchedule() {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.scheduleIndex++
	rs.enteredAt = time.Now() // reset timer for next interval
}

// getWaitDuration returns how long to wait before the next probe
func (rs *recoveryState) getWaitDuration() time.Duration {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.llmConfig == nil || rs.llmConfig.RecoveryConfig == nil || len(rs.llmConfig.RecoveryConfig.TestScheduleSeconds) == 0 {
		// Default schedule: 30 seconds
		return 30 * time.Second
	}

	schedule := rs.llmConfig.RecoveryConfig.TestScheduleSeconds
	idx := rs.scheduleIndex
	if idx >= len(schedule) {
		idx = len(schedule) - 1 // stay at last interval
	}

	return time.Duration(schedule[idx]) * time.Second
}

// shouldAbort returns true if we've exceeded the abort timeout
func (rs *recoveryState) shouldAbort() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if !rs.inRecovery {
		return false
	}

	if rs.llmConfig == nil || rs.llmConfig.RecoveryConfig == nil || rs.llmConfig.RecoveryConfig.AbortAfterSeconds == 0 {
		// Default: 12 hours
		return time.Since(rs.enteredAt) > 12*time.Hour
	}

	return time.Since(rs.enteredAt) > time.Duration(rs.llmConfig.RecoveryConfig.AbortAfterSeconds)*time.Second
}

// isInRecovery returns whether we're in recovery mode
func (rs *recoveryState) isInRecovery() bool {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.inRecovery
}

// getLLMID returns the LLM ID that triggered recovery
func (rs *recoveryState) getLLMID() string {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.llmID
}

// runBudget tracks LLM call budget for a run to prevent runaway costs
type runBudget struct {
	maxCalls  int64
	usedCalls int64 // accessed atomically
	exceeded  bool  // set when budget exceeded, prevents further calls
	bufferPct float64
}

// newRunBudget calculates an LLM call budget based on tasks and limits
// Formula per task: maxWorker + maxQA (QA calls include revision cycle)
// Then add a buffer percentage (default 10%)
func (r *Runner) newRunBudget(tasks []*global.Task, limits global.Limits, bufferPct float64) *runBudget {
	// Apply defaults if limits are zero
	limits = limits.WithDefaults()

	var totalCalls int64
	for _, task := range tasks {
		// Work phase: up to MaxWorker calls
		taskCalls := int64(limits.MaxWorker)

		// QA phase: if enabled, add QA calls
		if task.QA.Enabled {
			taskCalls += int64(limits.MaxQA)
		}

		totalCalls += taskCalls
	}

	// Add buffer
	if bufferPct <= 0 {
		bufferPct = 0.10 // default 10%
	}
	maxCalls := int64(float64(totalCalls) * (1.0 + bufferPct))

	return &runBudget{
		maxCalls:  maxCalls,
		bufferPct: bufferPct,
	}
}

// checkAndIncrement checks if budget allows another call and increments if so
// Returns true if call is allowed, false if budget exceeded
func (b *runBudget) checkAndIncrement() bool {
	if b == nil {
		return true // no budget means unlimited
	}
	if b.exceeded {
		return false
	}
	newCount := atomic.AddInt64(&b.usedCalls, 1)
	if newCount > b.maxCalls {
		b.exceeded = true
		return false
	}
	return true
}

// used returns current call count
func (b *runBudget) used() int64 {
	if b == nil {
		return 0
	}
	return atomic.LoadInt64(&b.usedCalls)
}

// ValidationErrorDetails contains detailed information about a schema validation failure
type ValidationErrorDetails struct {
	TaskID           int              `json:"task_id"`
	TaskUUID         string           `json:"task_uuid"`
	TaskTitle        string           `json:"task_title"`
	Timestamp        time.Time        `json:"timestamp"`
	Phase            string           `json:"phase"`                  // "worker" or "qa"
	ErrorType        string           `json:"error_type"`             // "schema_validation" or "parse_error"
	Summary          string           `json:"summary"`                // Brief human-readable summary
	ValidationErrors []string         `json:"validation_errors"`      // User-friendly error messages
	RawErrors        []string         `json:"raw_errors,omitempty"`   // Original error messages from validator
	LLMResponse      string           `json:"llm_response"`           // What the LLM actually returned
	LLMStderr        string           `json:"llm_stderr,omitempty"`   // Stderr from LLM command (if any)
	ExpectedSchema   string           `json:"expected_schema"`        // The JSON schema that was expected
	Invocation       int              `json:"invocation"`             // Which invocation number this was
	LLMModelID       string           `json:"llm_model_id,omitempty"` // Which LLM was used
	History          []global.Message `json:"history,omitempty"`      // Task execution history for debugging
}

// writeErrorFile writes detailed error information to a file in the results directory
// Returns the filename (not full path) for logging
func (r *Runner) writeErrorFile(project string, details *ValidationErrorDetails) (string, error) {
	resultsDir := r.tasks.GetResultsDir(project)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create results directory: %w", err)
	}

	filename := details.TaskUUID + "-error.json"
	filePath := filepath.Join(resultsDir, filename)

	data, err := json.MarshalIndent(details, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal error details: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write error file: %w", err)
	}

	return filename, nil
}

// formatValidationSummary creates a brief summary of validation errors
func formatValidationSummary(errors []string) string {
	if len(errors) == 0 {
		return "No validation errors"
	}
	if len(errors) == 1 {
		return errors[0]
	}
	return fmt.Sprintf("%d validation errors: %s", len(errors), errors[0])
}

// SchemaValidationError is returned when a response fails schema validation
// but can potentially be retried with the error feedback
type SchemaValidationError struct {
	Phase            string   // "worker" or "qa"
	ValidationErrors []string // User-friendly error messages
	ErrorFilename    string   // Name of the error details file
	CanRetry         bool     // Whether retry is allowed (under limits)
}

func (e *SchemaValidationError) Error() string {
	return fmt.Sprintf("%s schema validation failed (%d errors)", e.Phase, len(e.ValidationErrors))
}

// IsSchemaValidationError checks if an error is a SchemaValidationError
func IsSchemaValidationError(err error) (*SchemaValidationError, bool) {
	if sve, ok := err.(*SchemaValidationError); ok {
		return sve, true
	}
	return nil, false
}

// New creates a new Runner
func New(cfg *config.Config, logger *logging.Logger, lib *library.Service, playbooksSvc *playbooks.Service, refSvc *reference.Service, llmSvc *llm.Service, tasksSvc *tasks.Service, projectsSvc *projects.Service) *Runner {
	runnerConfig := cfg.Runner()

	// Create content loaders for report template loading
	// Playbook loader: parses "playbook-name/path/to/file" format
	playbookLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid playbook path: %s (expected playbook-name/path)", path)
		}
		item, err := playbooksSvc.GetFile(parts[0], parts[1], 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	// Reference loader
	referenceLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		item, err := refSvc.Get(path, 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	return &Runner{
		config:      cfg,
		logger:      logger,
		library:     lib,
		playbooks:   playbooksSvc,
		reference:   refSvc,
		llm:         llmSvc,
		tasks:       tasksSvc,
		projects:    projectsSvc,
		reporter:    reporting.New(logger, reporting.WithPlaybookLoader(playbookLoader), reporting.WithReferenceLoader(referenceLoader)),
		validator:   templates.New(logger),
		rateLimiter: NewRateLimiter(runnerConfig.RateLimit.MaxRequests, runnerConfig.RateLimit.PeriodSeconds),
	}
}

// logToProject appends a message to the project log (best effort)
func (r *Runner) logToProject(project, message string) {
	if err := r.tasks.AppendLog(project, message); err != nil {
		r.logger.Warnf("Failed to append to project log: %v", err)
	}
}

// logTaskFinished logs a final "Finished" message when a task reaches a terminal state.
// This is only called for terminal states (done, failed, escalate), not for tasks that will be retried.
func (r *Runner) logTaskFinished(project string, task *global.Task) {
	// Determine the final status string
	var finalStatus string

	// Check if task is in a terminal state
	if task.Work.Status == global.ExecutionStatusWaiting || task.Work.Status == global.ExecutionStatusRetry {
		// Task will be retried, not finished yet
		return
	}

	if task.Work.Status == global.ExecutionStatusFailed {
		finalStatus = "failed"
	} else if task.Work.Status == global.ExecutionStatusDone {
		// Check QA verdict if QA was enabled
		if task.QA.Enabled {
			switch task.QA.Verdict {
			case global.QAVerdictEscalate:
				finalStatus = "escalate"
			case global.QAVerdictPass:
				finalStatus = "done"
			case global.QAVerdictFail:
				// QA failed but task is done - this means max QA retries reached
				finalStatus = "done (QA failed)"
			default:
				finalStatus = "done"
			}
		} else {
			finalStatus = "done"
		}
	} else {
		// Unknown or processing state - don't log finished
		return
	}

	r.logger.Infof("Task %d: Finished with status %s", task.ID, finalStatus)
	r.logToProject(project, fmt.Sprintf("Task %d: Finished with status %s", task.ID, finalStatus))
}

// recordHistoryPrompt records a prompt message to task history
func (r *Runner) recordHistoryPrompt(taskUUID, role, prompt, llmID string, invocation int) {
	msg := global.Message{
		Timestamp:  time.Now(),
		Role:       role,
		Invocation: invocation,
		LLMModelID: llmID,
		Prompt:     prompt,
		Type:       "prompt", // Legacy field for compatibility
		Content:    prompt,   // Legacy field for compatibility
	}

	existing, _ := r.taskHistory.LoadOrStore(taskUUID, []global.Message{})
	history := existing.([]global.Message)
	history = append(history, msg)
	r.taskHistory.Store(taskUUID, history)
}

// recordHistoryResponse records a response message to task history
func (r *Runner) recordHistoryResponse(taskUUID, role string, result *llm.DispatchResult, llmID string, invocation int) {
	exitCode := 0
	var stdout, stderr string
	var responseSize int

	if result != nil {
		exitCode = result.ExitCode
		stdout = result.Stdout
		stderr = result.Stderr
		responseSize = result.ResponseSize
	}

	msg := global.Message{
		Timestamp:    time.Now(),
		Role:         role,
		Invocation:   invocation,
		LLMModelID:   llmID,
		ExitCode:     &exitCode,
		Stdout:       stdout,
		Stderr:       stderr,
		ResponseSize: responseSize,
		Type:         "response",    // Legacy field for compatibility
		Content:      stdout,        // Legacy field for compatibility
	}

	existing, _ := r.taskHistory.LoadOrStore(taskUUID, []global.Message{})
	history := existing.([]global.Message)
	history = append(history, msg)
	r.taskHistory.Store(taskUUID, history)
}

// recordHistoryError records an infrastructure error to task history
func (r *Runner) recordHistoryError(taskUUID, role, errorMsg, llmID string, invocation int) {
	msg := global.Message{
		Timestamp:  time.Now(),
		Role:       role,
		Invocation: invocation,
		LLMModelID: llmID,
		Error:      errorMsg,
		Type:       "error",  // Legacy field for compatibility
		Content:    errorMsg, // Legacy field for compatibility
	}

	existing, _ := r.taskHistory.LoadOrStore(taskUUID, []global.Message{})
	history := existing.([]global.Message)
	history = append(history, msg)
	r.taskHistory.Store(taskUUID, history)
}

// recordHistory appends a message to task history (legacy function for compatibility)
func (r *Runner) recordHistory(project, taskUUID, role, msgType, content, llmID string, invocation int, stderr ...string) {
	msg := global.Message{
		Timestamp:  time.Now(),
		Role:       role,
		Type:       msgType,
		Content:    content,
		LLMModelID: llmID,
		Invocation: invocation,
	}
	// Include stderr if provided
	if len(stderr) > 0 && stderr[0] != "" {
		msg.Stderr = stderr[0]
	}

	// Append to in-memory history (will be saved to result file)
	existing, _ := r.taskHistory.LoadOrStore(taskUUID, []global.Message{})
	history := existing.([]global.Message)
	history = append(history, msg)
	r.taskHistory.Store(taskUUID, history)
}

// collectUniqueLLMs collects unique LLM IDs from tasks (worker + QA)
func (r *Runner) collectUniqueLLMs(tasks []*global.Task) []string {
	seen := make(map[string]bool)
	var llms []string

	// Get default LLM
	defaultLLM := r.config.DefaultLLM()
	if defaultLLM == "" {
		enabledLLMs := r.config.EnabledLLMs()
		if len(enabledLLMs) > 0 {
			defaultLLM = enabledLLMs[0].ID
		}
	}

	for _, task := range tasks {
		// Worker LLM
		workerLLM := task.Work.LLMModelID
		if workerLLM == "" || workerLLM == "default" {
			workerLLM = defaultLLM
		}
		if workerLLM != "" && !seen[workerLLM] {
			seen[workerLLM] = true
			llms = append(llms, workerLLM)
		}

		// QA LLM (if enabled)
		if task.QA.Enabled {
			qaLLM := task.QA.LLMModelID
			if qaLLM == "" || qaLLM == "default" {
				qaLLM = defaultLLM
			}
			if qaLLM != "" && !seen[qaLLM] {
				seen[qaLLM] = true
				llms = append(llms, qaLLM)
			}
		}
	}

	return llms
}

// getTaskHistory retrieves accumulated history for a task
func (r *Runner) getTaskHistory(taskUUID string) []global.Message {
	if existing, ok := r.taskHistory.Load(taskUUID); ok {
		return existing.([]global.Message)
	}
	return nil
}

// clearTaskHistory removes accumulated history for a task (called after saving to result file)
func (r *Runner) clearTaskHistory(taskUUID string) {
	r.taskHistory.Delete(taskUUID)
}

// TaskStatusResult represents the status of tasks in a project
type TaskStatusResult struct {
	Project       string           `json:"project"`
	TotalTasks    int              `json:"total_tasks"`
	Pending       int              `json:"pending"`
	InProgress    int              `json:"in_progress"`
	Done          int              `json:"done"`
	Failed        int              `json:"failed"`
	RunInProgress bool             `json:"run_in_progress"`
	Tasks         []TaskStatusInfo `json:"tasks"`
}

// TaskStatusInfo represents basic task information for status checking
type TaskStatusInfo struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// GetTaskStatus returns the current status of tasks in a project
func (r *Runner) GetTaskStatus(project, path, taskType string) (*TaskStatusResult, error) {
	if !r.tasks.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	// List task sets at path prefix (empty means all)
	taskSetList, err := r.tasks.ListTaskSets(project, path)
	if err != nil {
		return nil, fmt.Errorf("failed to list task sets: %w", err)
	}

	// Aggregate task counts across all task sets
	result := &TaskStatusResult{
		Project: project,
		Tasks:   []TaskStatusInfo{},
	}

	for _, taskSet := range taskSetList.TaskSets {
		for _, task := range taskSet.Tasks {
			// Apply type filter if provided
			if taskType != "" && task.Type != taskType {
				continue
			}

			result.TotalTasks++

			// Count by status
			switch task.Work.Status {
			case global.ExecutionStatusWaiting, global.ExecutionStatusRetry:
				result.Pending++
			case global.ExecutionStatusProcessing:
				result.InProgress++
			case global.ExecutionStatusDone:
				result.Done++
			case global.ExecutionStatusFailed:
				result.Failed++
			}

			// Add task info
			result.Tasks = append(result.Tasks, TaskStatusInfo{
				ID:     task.ID,
				Status: task.Work.Status,
			})
		}
	}

	// Check if a run is in progress
	_, runInProgress := r.runningProjects.Load(project)
	result.RunInProgress = runInProgress

	return result, nil
}

// Run executes eligible tasks for a project in the background
// Returns immediately with the count of tasks queued
func (r *Runner) Run(ctx context.Context, req *global.RunRequest) (*global.RunResult, error) {
	// Validate project exists
	if !r.tasks.ProjectExists(req.Project) {
		return nil, fmt.Errorf("project not found: %s", req.Project)
	}

	// Validate disclaimer_template is configured
	proj, err := r.projects.Get(req.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	if proj.DisclaimerTemplate == "" {
		return nil, fmt.Errorf("disclaimer_template is not configured for project %s: update project with disclaimer_template set to a playbook path or 'none'", req.Project)
	}
	// If "none", the report generation will use empty string (current behavior)
	// If a path, validate the file exists before starting
	if proj.DisclaimerTemplate != "none" {
		parts := strings.SplitN(proj.DisclaimerTemplate, "/", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid disclaimer_template format: must be 'playbook-name/path/to/file.md', got: %s", proj.DisclaimerTemplate)
		}
		fullPath := filepath.Join(r.config.PlaybooksDir(), parts[0], parts[1])
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("disclaimer template not found: %s", proj.DisclaimerTemplate)
		}
	}

	// Check if a run is already in progress
	_, alreadyRunning := r.runningProjects.LoadOrStore(req.Project, true)
	if alreadyRunning {
		return &global.RunResult{
			Project:    req.Project,
			Path:       req.Path,
			TasksFound: 0,
			Message:    fmt.Sprintf("a run is already in progress for project: %s", req.Project),
		}, nil
	}

	// List task sets at path (empty means all)
	taskSetList, err := r.tasks.ListTaskSets(req.Project, req.Path)
	if err != nil {
		r.runningProjects.Delete(req.Project)
		return nil, fmt.Errorf("failed to list task sets: %w", err)
	}

	// Validate templates for all task sets before starting
	var templateErrors []string
	for _, taskSet := range taskSetList.TaskSets {
		errors := r.validateTaskSetTemplates(req.Project, taskSet)
		for _, e := range errors {
			templateErrors = append(templateErrors, fmt.Sprintf("task set '%s': %s", taskSet.Path, e))
		}
	}
	if len(templateErrors) > 0 {
		r.runningProjects.Delete(req.Project)
		return nil, fmt.Errorf("template validation failed:\n  - %s", strings.Join(templateErrors, "\n  - "))
	}

	// Collect eligible tasks from all task sets
	var eligibleTasks []*global.Task
	taskSetPaths := make(map[string]string) // map task UUID to task set path

	for _, taskSet := range taskSetList.TaskSets {
		for i := range taskSet.Tasks {
			task := &taskSet.Tasks[i]

			// Check if eligible (waiting or retry status)
			if task.Work.Status != global.ExecutionStatusWaiting && task.Work.Status != global.ExecutionStatusRetry {
				continue
			}

			// Apply type filter if provided
			if req.Type != "" && task.Type != req.Type {
				continue
			}

			eligibleTasks = append(eligibleTasks, task)
			taskSetPaths[task.UUID] = taskSet.Path
		}
	}

	// Create result
	result := &global.RunResult{
		Project:    req.Project,
		Path:       req.Path,
		TasksFound: len(eligibleTasks),
	}

	// If no tasks found, release lock and return
	if len(eligibleTasks) == 0 {
		r.runningProjects.Delete(req.Project)
		result.Message = "no eligible tasks found"
		return result, nil
	}

	// Validate timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = global.DefaultTimeout
	}

	// Prepare execution parameters
	execParams := &runExecutionParams{
		ctx:           ctx,
		req:           req,
		taskSetList:   taskSetList,
		eligibleTasks: eligibleTasks,
		result:        result,
		timeout:       timeout,
	}

	if req.Wait {
		// Synchronous execution - block until complete
		result.Message = fmt.Sprintf("executing %d tasks synchronously", len(eligibleTasks))
		r.activeRuns.Add(1)
		r.executeRun(execParams)
		r.activeRuns.Done()
		r.runningProjects.Delete(req.Project)
	} else {
		// Async execution - return immediately
		result.Message = fmt.Sprintf("%d tasks queued for execution", len(eligibleTasks))
		r.activeRuns.Add(1)
		go func() {
			defer r.activeRuns.Done()
			defer r.runningProjects.Delete(req.Project)
			r.executeRun(execParams)
		}()
	}

	return result, nil
}

// runExecutionParams holds parameters for task execution
type runExecutionParams struct {
	ctx           context.Context
	req           *global.RunRequest
	taskSetList   *tasks.TaskSetListResult
	eligibleTasks []*global.Task
	result        *global.RunResult
	timeout       int
}

// executeRun performs the actual task execution (shared between sync and async modes)
func (r *Runner) executeRun(params *runExecutionParams) {
	// Get limits from first task set or use config defaults
	var limits global.Limits
	if len(params.taskSetList.TaskSets) > 0 {
		limits = params.taskSetList.TaskSets[0].Limits
	}
	if limits.MaxWorker == 0 && limits.MaxQA == 0 {
		// Fall back to config limits
		limits = r.config.Runner().Limits
	}
	limits = limits.WithDefaults()

	// Calculate LLM call budget to prevent runaway costs
	budget := r.newRunBudget(params.eligibleTasks, limits, 0.10)
	r.logger.Infof("Starting run for project %s: %d eligible tasks, LLM budget: %d calls (limits: worker=%d, qa=%d)",
		params.req.Project, len(params.eligibleTasks), budget.maxCalls, limits.MaxWorker, limits.MaxQA)
	r.logToProject(params.req.Project, fmt.Sprintf("Run started: %d eligible tasks, LLM call budget: %d (limits: worker=%d, qa=%d)",
		len(params.eligibleTasks), budget.maxCalls, limits.MaxWorker, limits.MaxQA))

	// Pre-flight LLM check: test all LLMs that will be used
	llmsToTest := r.collectUniqueLLMs(params.eligibleTasks)
	if len(llmsToTest) > 0 {
		r.logger.Infof("Pre-flight check: testing %d LLM(s) (%s)", len(llmsToTest), strings.Join(llmsToTest, ", "))
		r.logToProject(params.req.Project, fmt.Sprintf("Pre-flight check: testing %d LLM(s)", len(llmsToTest)))

		for _, llmID := range llmsToTest {
			available, err := r.llm.TestLLM(llmID)
			if err != nil {
				r.logger.Errorf("Pre-flight check failed for %s: %v", llmID, err)
				r.logToProject(params.req.Project, fmt.Sprintf("Pre-flight check failed for %s: %v", llmID, err))
				return
			}
			if !available {
				r.logger.Errorf("Pre-flight check: LLM %s is not available", llmID)
				r.logToProject(params.req.Project, fmt.Sprintf("Pre-flight check: LLM %s is not available (possibly rate limited)", llmID))
				return
			}
			r.logger.Infof("Pre-flight check: %s OK", llmID)
		}
		r.logger.Infof("Pre-flight check: all LLMs available, starting %d tasks", len(params.eligibleTasks))
		r.logToProject(params.req.Project, fmt.Sprintf("Pre-flight check passed, starting %d tasks", len(params.eligibleTasks)))
	}

	// Determine parallel mode: req.Parallel overrides taskset.Parallel
	runParallel := false
	if params.req.Parallel != nil {
		// Explicit override from task_run
		runParallel = *params.req.Parallel
	} else if len(params.taskSetList.TaskSets) > 0 {
		// Use taskset setting
		runParallel = params.taskSetList.TaskSets[0].Parallel
	}

	if runParallel {
		// Get max concurrency from config
		maxConcurrent := r.config.Runner().MaxConcurrent
		r.runParallel(params.ctx, params.req.Project, params.req.Path, params.eligibleTasks, params.result, maxConcurrent, params.timeout, budget, limits)
	} else {
		r.runSequential(params.ctx, params.req.Project, params.req.Path, params.eligibleTasks, params.result, params.timeout, budget, limits)
	}

	// Log budget usage
	r.logger.Infof("Run completed for project %s: executed=%d, succeeded=%d, failed=%d, skipped=%d, LLM calls: %d/%d",
		params.req.Project, params.result.TasksExecuted, params.result.TasksSucceeded, params.result.TasksFailed, params.result.TasksSkipped,
		budget.used(), budget.maxCalls)
	completionMsg := fmt.Sprintf("Run completed: executed=%d, succeeded=%d, failed=%d, skipped=%d, LLM calls: %d/%d",
		params.result.TasksExecuted, params.result.TasksSucceeded, params.result.TasksFailed, params.result.TasksSkipped,
		budget.used(), budget.maxCalls)
	if budget.exceeded {
		completionMsg += " [BUDGET EXCEEDED - some tasks skipped]"
	}
	r.logToProject(params.req.Project, completionMsg)

	// Auto-generate report after run completes
	if _, err := r.generateAndSaveReport(params.req.Project, params.req.Path); err != nil {
		r.logger.Errorf("Failed to generate report for project %s: %v", params.req.Project, err)
	}
}

// Wait blocks until all active runs complete. Used for graceful shutdown.
func (r *Runner) Wait() {
	r.activeRuns.Wait()
}

// IsRunning returns true if any runs are currently in progress.
func (r *Runner) IsRunning() bool {
	running := false
	r.runningProjects.Range(func(_, _ interface{}) bool {
		running = true
		return false // stop iteration
	})
	return running
}

// getTasksNeedingRetry returns tasks that are in waiting or retry status and need re-processing.
// This is used to find tasks that failed schema validation and were set back to waiting.
func (r *Runner) getTasksNeedingRetry(project, path string) []*global.Task {
	taskSetList, err := r.tasks.ListTaskSets(project, path)
	if err != nil {
		r.logger.Warnf("Failed to list task sets for retry check: %v", err)
		return nil
	}

	var tasksNeedingRetry []*global.Task
	for _, taskSet := range taskSetList.TaskSets {
		for i := range taskSet.Tasks {
			task := &taskSet.Tasks[i]
			if task.Work.Status == global.ExecutionStatusWaiting || task.Work.Status == global.ExecutionStatusRetry {
				tasksNeedingRetry = append(tasksNeedingRetry, task)
			}
		}
	}

	return tasksNeedingRetry
}

// runSequential executes tasks one at a time.
// In sequential mode, tasks are assumed to be dependent on previous tasks completing.
// If a task is not done (failed, waiting, etc.), the pass ends and we move to the next round.
func (r *Runner) runSequential(ctx context.Context, project, path string, tasks []*global.Task, result *global.RunResult, timeout int, budget *runBudget, limits global.Limits) {
	maxRounds := r.config.Runner().MaxRounds
	runnerConfig := r.config.Runner()
	roundDelay := time.Duration(runnerConfig.RoundDelaySeconds) * time.Second
	recovery := newRecoveryState()

	// Process tasks in rounds until no more need processing
	for round := 1; round <= maxRounds; round++ {
		// Apply round delay before second and subsequent rounds
		if round > 1 && roundDelay > 0 {
			r.logger.Infof("Round delay: waiting %v before starting round %d", roundDelay, round)
			r.logToProject(project, fmt.Sprintf("Round delay: waiting %v before round %d", roundDelay, round))
			select {
			case <-ctx.Done():
				return
			case <-time.After(roundDelay):
			}
		}

		var tasksToProcess []*global.Task

		if round == 1 {
			// First round uses the initial task list
			tasksToProcess = tasks
		} else {
			// Subsequent rounds re-fetch tasks in waiting status
			tasksToProcess = r.getTasksNeedingRetry(project, path)
			if len(tasksToProcess) == 0 {
				break // No more tasks need processing
			}
			r.logger.Infof("Round %d/%d: %d task(s) need processing", round, maxRounds, len(tasksToProcess))
			r.logToProject(project, fmt.Sprintf("Round %d/%d: %d task(s) need processing", round, maxRounds, len(tasksToProcess)))
		}

		passComplete := true // assume we'll complete the pass unless a task isn't done

		for _, task := range tasksToProcess {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Check if we should abort due to recovery timeout
			if recovery.shouldAbort() {
				r.logger.Warnf("Recovery timeout reached, aborting run. Uncompleted tasks remain in waiting status.")
				r.logToProject(project, "Recovery timeout reached, aborting run. Uncompleted tasks remain in waiting status.")
				return
			}

			// Handle recovery mode - wait and probe before continuing
			if recovery.isInRecovery() {
				if !r.handleRecovery(ctx, project, recovery) {
					// Recovery failed or aborted
					return
				}
			}

			// Check if budget exceeded before starting task
			if budget != nil && budget.exceeded {
				r.logger.Warnf("Task %d: Skipping - LLM budget exceeded", task.ID)
				r.logToProject(project, fmt.Sprintf("Task %d: Skipped - LLM budget exceeded", task.ID))
				result.TasksSkipped++
				passComplete = false
				break // End this pass - budget exceeded
			}

			// Need to find the task set path for this task
			taskInfo, taskSetPath, err := r.tasks.GetTask(project, task.UUID)
			if err != nil {
				r.logger.Errorf("Task %d: Failed to get task set path: %v", task.ID, err)
				result.TasksSkipped++
				passComplete = false
				break // End this pass - can't proceed without task info
			}

			// Execute the task
			r.executeTaskWithRecovery(ctx, project, taskSetPath, taskInfo, result, timeout, budget, limits, recovery)

			// Refresh task status after execution
			updatedTask, _, err := r.tasks.GetTask(project, task.UUID)
			if err != nil {
				r.logger.Errorf("Task %d: Failed to refresh task status: %v", task.ID, err)
				passComplete = false
				break
			}

			// Sequential mode: if task is not done, end this pass
			if updatedTask.Work.Status != global.ExecutionStatusDone {
				passComplete = false
				break // End this pass - task not complete, will retry next round
			}
		}

		// If we completed the pass with all tasks done, we're finished
		if passComplete {
			remaining := r.getTasksNeedingRetry(project, path)
			if len(remaining) == 0 {
				break // All done!
			}
		}
	}

	// Check if max rounds reached with tasks still waiting
	// Note: We don't mark tasks as failed here - they remain in waiting status
	// so a future run can pick them up
	remainingTasks := r.getTasksNeedingRetry(project, path)
	if len(remainingTasks) > 0 {
		r.logger.Warnf("Max rounds (%d) reached with %d task(s) still waiting", maxRounds, len(remainingTasks))
		r.logToProject(project, fmt.Sprintf("Max rounds (%d) reached with %d task(s) still waiting. Tasks remain in waiting status for future runs.", maxRounds, len(remainingTasks)))
	}
}

// runParallel executes tasks concurrently with a worker pool.
// In parallel mode, tasks are independent and can run concurrently.
// If a task fails, other tasks continue. Recovery mode is checked between rounds.
func (r *Runner) runParallel(ctx context.Context, project, path string, tasks []*global.Task, result *global.RunResult, maxConcurrent int, timeout int, budget *runBudget, limits global.Limits) {
	var mu sync.Mutex
	sem := make(chan struct{}, maxConcurrent)
	maxRounds := r.config.Runner().MaxRounds
	runnerConfig := r.config.Runner()
	roundDelay := time.Duration(runnerConfig.RoundDelaySeconds) * time.Second
	recovery := newRecoveryState()

	// Process tasks in rounds until no more need processing
	for round := 1; round <= maxRounds; round++ {
		// Apply round delay before second and subsequent rounds
		if round > 1 && roundDelay > 0 {
			r.logger.Infof("Round delay: waiting %v before starting round %d", roundDelay, round)
			r.logToProject(project, fmt.Sprintf("Round delay: waiting %v before round %d", roundDelay, round))
			select {
			case <-ctx.Done():
				return
			case <-time.After(roundDelay):
			}
		}

		// Check if we should abort due to recovery timeout
		if recovery.shouldAbort() {
			r.logger.Warnf("Recovery timeout reached, aborting run. Uncompleted tasks remain in waiting status.")
			r.logToProject(project, "Recovery timeout reached, aborting run. Uncompleted tasks remain in waiting status.")
			return
		}

		// Handle recovery mode - wait and probe before continuing with this round
		if recovery.isInRecovery() {
			if !r.handleRecovery(ctx, project, recovery) {
				// Recovery failed or aborted
				return
			}
		}

		var tasksToProcess []*global.Task

		if round == 1 {
			// First round uses the initial task list
			tasksToProcess = tasks
		} else {
			// Subsequent rounds re-fetch tasks in waiting status
			tasksToProcess = r.getTasksNeedingRetry(project, path)
			if len(tasksToProcess) == 0 {
				break // No more tasks need processing
			}
			r.logger.Infof("Round %d/%d: %d task(s) need processing", round, maxRounds, len(tasksToProcess))
			r.logToProject(project, fmt.Sprintf("Round %d/%d: %d task(s) need processing", round, maxRounds, len(tasksToProcess)))
		}

		var wg sync.WaitGroup

		for _, task := range tasksToProcess {
			select {
			case <-ctx.Done():
				wg.Wait() // Wait for in-flight tasks before returning
				return
			default:
			}

			// Check if budget exceeded before starting task
			if budget != nil && budget.exceeded {
				r.logger.Warnf("Task %d: Skipping - LLM budget exceeded", task.ID)
				r.logToProject(project, fmt.Sprintf("Task %d: Skipped - LLM budget exceeded", task.ID))
				mu.Lock()
				result.TasksSkipped++
				mu.Unlock()
				continue
			}

			wg.Add(1)
			sem <- struct{}{}

			go func(t *global.Task) {
				defer wg.Done()
				defer func() { <-sem }()

				// Need to find the task set path for this task
				taskInfo, taskSetPath, err := r.tasks.GetTask(project, t.UUID)
				if err != nil {
					r.logger.Errorf("Task %d: Failed to get task set path: %v", t.ID, err)
					mu.Lock()
					result.TasksSkipped++
					mu.Unlock()
					return
				}

				localResult := &global.RunResult{}
				r.executeTask(ctx, project, taskSetPath, taskInfo, localResult, timeout, budget, limits)

				// Merge results
				mu.Lock()
				result.TasksExecuted += localResult.TasksExecuted
				result.TasksSucceeded += localResult.TasksSucceeded
				result.TasksFailed += localResult.TasksFailed
				result.TasksSkipped += localResult.TasksSkipped
				mu.Unlock()

				// Check if task failed and we should enter recovery mode
				// This is checked after task completion to allow other workers to finish
				updatedTask, _, getErr := r.tasks.GetTask(project, t.UUID)
				if getErr == nil && updatedTask.Work.Status != global.ExecutionStatusDone {
					llmID := t.Work.LLMModelID
					if llmID == "" {
						llmID = r.config.DefaultLLM()
						if llmID == "" {
							enabledLLMs := r.config.EnabledLLMs()
							if len(enabledLLMs) > 0 {
								llmID = enabledLLMs[0].ID
							}
						}
					}
					llmConfig := r.llm.GetLLM(llmID)
					if llmConfig != nil && llmConfig.RecoveryConfig != nil {
						r.logger.Infof("Task %d: Failed - entering recovery mode for LLM %s", t.ID, llmID)
						r.logToProject(project, fmt.Sprintf("Task %d: Failed - entering recovery mode for LLM %s", t.ID, llmID))
						recovery.enterRecovery(llmID, llmConfig)
					}
				}
			}(task)
		}

		wg.Wait()
	}

	// Check if max rounds reached with tasks still waiting
	// Note: We don't mark tasks as failed here - they remain in waiting status
	// so a future run can pick them up
	remainingTasks := r.getTasksNeedingRetry(project, path)
	if len(remainingTasks) > 0 {
		r.logger.Warnf("Max rounds (%d) reached with %d task(s) still waiting", maxRounds, len(remainingTasks))
		r.logToProject(project, fmt.Sprintf("Max rounds (%d) reached with %d task(s) still waiting. Tasks remain in waiting status for future runs.", maxRounds, len(remainingTasks)))
	}
}

// handleRecovery waits for recovery mode to complete by probing the LLM.
// Returns true if recovery succeeded (LLM is available), false if aborted or cancelled.
func (r *Runner) handleRecovery(ctx context.Context, project string, recovery *recoveryState) bool {
	for recovery.isInRecovery() {
		// Check abort timeout
		if recovery.shouldAbort() {
			r.logger.Warnf("Project %s: Recovery timeout exceeded, aborting run", project)
			r.logToProject(project, "Recovery timeout exceeded, aborting run. Remaining tasks left in waiting status.")
			return false
		}

		// Wait for the scheduled duration
		waitDuration := recovery.getWaitDuration()
		llmID := recovery.getLLMID()
		r.logger.Infof("Project %s: Recovery mode - waiting %v before probing LLM %s", project, waitDuration, llmID)
		r.logToProject(project, fmt.Sprintf("Recovery mode: waiting %v before probing LLM %s", waitDuration, llmID))

		select {
		case <-ctx.Done():
			r.logger.Infof("Project %s: Run cancelled during recovery", project)
			return false
		case <-time.After(waitDuration):
		}

		// Probe the LLM with test prompt
		llmConfig := r.llm.GetLLM(llmID)
		if llmConfig == nil {
			r.logger.Errorf("Project %s: Recovery failed - LLM %s not found", project, llmID)
			recovery.exitRecovery()
			return false
		}

		testPrompt := "test"
		if llmConfig.RecoveryConfig != nil && llmConfig.RecoveryConfig.TestPrompt != "" {
			testPrompt = llmConfig.RecoveryConfig.TestPrompt
		}

		r.logger.Infof("Project %s: Probing LLM %s with test prompt", project, llmID)
		r.logToProject(project, fmt.Sprintf("Probing LLM %s", llmID))

		req := &llm.DispatchRequest{
			LLMID:  llmID,
			Prompt: testPrompt,
		}
		result, err := r.llm.Dispatch(req)

		if err != nil {
			r.logger.Warnf("Project %s: Probe failed (infrastructure error): %v", project, err)
			r.logToProject(project, fmt.Sprintf("Probe failed: %v", err))
			recovery.advanceSchedule()
			continue
		}

		if result.ExitCode != 0 {
			r.logger.Warnf("Project %s: Probe failed (exit code %d)", project, result.ExitCode)
			r.logToProject(project, fmt.Sprintf("Probe failed: exit code %d", result.ExitCode))
			recovery.advanceSchedule()
			continue
		}

		// Probe succeeded - exit recovery mode
		r.logger.Infof("Project %s: Probe succeeded - LLM %s is available", project, llmID)
		r.logToProject(project, fmt.Sprintf("Probe succeeded - LLM %s is available, resuming tasks", llmID))
		recovery.exitRecovery()
		return true
	}

	return true
}

// executeTaskWithRecovery executes a task and enters recovery mode if it fails.
// This wrapper is used in sequential mode where we need to pause on failures.
func (r *Runner) executeTaskWithRecovery(ctx context.Context, project, path string, task *global.Task, result *global.RunResult, timeout int, budget *runBudget, limits global.Limits, recovery *recoveryState) {
	// Check for cancellation
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Execute the task
	r.executeTask(ctx, project, path, task, result, timeout, budget, limits)

	// Check if the task failed - if so, we may need to enter recovery mode
	updatedTask, _, err := r.tasks.GetTask(project, task.UUID)
	if err != nil {
		r.logger.Warnf("Task %d: Failed to get task status after execution: %v", task.ID, err)
		return
	}

	// If task is not done (failed or retry status), check if we should enter recovery
	if updatedTask.Work.Status != global.ExecutionStatusDone {
		llmID := task.Work.LLMModelID
		if llmID == "" {
			llmID = r.config.DefaultLLM()
			if llmID == "" {
				enabledLLMs := r.config.EnabledLLMs()
				if len(enabledLLMs) > 0 {
					llmID = enabledLLMs[0].ID
				}
			}
		}

		llmConfig := r.llm.GetLLM(llmID)
		if llmConfig != nil && llmConfig.RecoveryConfig != nil {
			r.logger.Infof("Task %d: Failed - entering recovery mode for LLM %s", task.ID, llmID)
			r.logToProject(project, fmt.Sprintf("Task %d: Failed - entering recovery mode for LLM %s", task.ID, llmID))
			recovery.enterRecovery(llmID, llmConfig)
		}
	}
}

// executeTask executes a single task
func (r *Runner) executeTask(_ context.Context, project, path string, task *global.Task, result *global.RunResult, timeout int, budget *runBudget, limits global.Limits) {
	// Panic recovery to prevent crashes
	defer func() {
		if rec := recover(); rec != nil {
			errMsg := fmt.Sprintf("PANIC in task execution: %v", rec)
			r.logger.Errorf("Task %d: %s", task.ID, errMsg)
			r.logToProject(project, fmt.Sprintf("Task %d crashed: %v", task.ID, rec))
			r.finishTask(project, path, task, "", errMsg, "", "", result, limits)
		}
	}()

	// Wait for rate limiter
	r.logger.Infof("Task %d: Waiting for rate limiter", task.ID)
	r.rateLimiter.Wait()
	r.logger.Infof("Task %d: Rate limiter passed", task.ID)

	// Check if work has already completed successfully (has results file with worker response)
	// This prevents re-calling the worker LLM when only QA needs to be retried
	resultsDir := r.tasks.GetResultsDir(project)
	resultPath := filepath.Join(resultsDir, task.UUID+".json")
	if data, err := os.ReadFile(resultPath); err == nil {
		var existingResult global.TaskResult
		if err := json.Unmarshal(data, &existingResult); err == nil && existingResult.Worker.Response != "" {
			// Work already completed - skip to QA workflow if needed
			r.logger.Infof("Task %d: Work already completed (found existing result), checking QA status", task.ID)
			r.logToProject(project, fmt.Sprintf("Task %d: Work already completed, checking QA", task.ID))

			// Update local task with the response for QA
			task.Work.Status = global.ExecutionStatusDone

			// Check if QA needs to be run
			if task.QA.Enabled && task.QA.Status != global.ExecutionStatusDone {
				r.logger.Infof("Task %d: QA enabled and not complete, starting QA workflow", task.ID)
				r.executeQAWorkflow(project, path, task, result, timeout, budget, limits)
			}
			return
		}
	}

	// Determine which LLM will be used
	llmID := task.Work.LLMModelID
	if llmID == "" || llmID == "default" {
		// Use configured default_llm if available
		defaultLLM := r.config.DefaultLLM()
		if defaultLLM != "" {
			llmID = defaultLLM
			r.logger.Infof("Task %d: Using configured default LLM: %s", task.ID, llmID)
		} else {
			// Fallback to first enabled LLM
			r.logger.Infof("Task %d: No default_llm configured, using first enabled LLM", task.ID)
			enabledLLMs := r.config.EnabledLLMs()
			if len(enabledLLMs) > 0 {
				llmID = enabledLLMs[0].ID
				r.logger.Infof("Task %d: Selected LLM: %s", task.ID, llmID)
			} else {
				r.logToProject(project, fmt.Sprintf("Task %d: Failed - no LLMs are enabled", task.ID))
				r.logger.Errorf("Task %d: Failed - no LLMs are enabled", task.ID)
				result.TasksSkipped++
				return
			}
		}
		// Store resolved LLM ID for result file
		task.Work.LLMModelID = llmID
	}

	// Log task start
	r.logToProject(project, fmt.Sprintf("Task %d: Started, LLM: %s", task.ID, llmID))
	r.logger.Infof("Task %d: Started, LLM: %s", task.ID, llmID)

	// Update task metadata but keep status as 'waiting' until fully complete
	// This ensures restarts will pick up interrupted tasks
	r.logger.Infof("Task %d: Updating task metadata (status stays waiting until complete)", task.ID)
	now := time.Now()
	task.Work.Invocations++
	task.Work.LastAttemptAt = &now
	task.UpdatedAt = now

	// Save task metadata update (status remains 'waiting')
	updates := map[string]interface{}{
		"work": map[string]interface{}{
			"invocations":     task.Work.Invocations,
			"last_attempt_at": &now,
		},
	}
	if _, err := r.tasks.UpdateTask(project, task.UUID, updates); err != nil {
		r.logger.Warnf("Task %d: Failed to save task metadata: %v", task.ID, err)
	}

	result.TasksExecuted++
	r.logger.Infof("Task %d: Beginning execution", task.ID)

	// Build prompt from instructions_file, instructions, prompt
	r.logger.Infof("Task %d: Building prompt", task.ID)
	fullPrompt, err := r.buildPrompt(project, path, task)
	if err != nil {
		r.logger.Errorf("Task %d: Failed to build prompt: %v", task.ID, err)
		r.logToProject(project, fmt.Sprintf("Task %d: Failed to build prompt: %v", task.ID, err))
		r.recordHistory(project, task.UUID, "system", "error", fmt.Sprintf("Failed to build prompt: %v", err), "", task.Work.Invocations)
		r.finishTask(project, path, task, "", err.Error(), "", "", result, limits)
		return
	}
	promptSize := len(fullPrompt)
	r.logger.Infof("Task %d: Prompt built (%d bytes)", task.ID, promptSize)

	// Record worker prompt in history
	r.recordHistory(project, task.UUID, "worker", "prompt", fullPrompt, llmID, task.Work.Invocations)

	// Build dispatch options with timeout if specified
	var options *llm.DispatchOptions
	timeoutSeconds := global.DefaultTimeout
	if timeout > 0 {
		timeoutSeconds = timeout
		options = &llm.DispatchOptions{
			Timeout: timeout,
		}
	}

	// Check budget before LLM call
	if !budget.checkAndIncrement() {
		r.logger.Warnf("Task %d: LLM budget exceeded, skipping", task.ID)
		r.logToProject(project, fmt.Sprintf("Task %d: LLM budget exceeded, skipping", task.ID))
		r.finishTask(project, path, task, "", "LLM budget exceeded", fullPrompt, "", result, limits)
		return
	}

	// Call LLM - get exec info for detailed logging
	execInfo := r.llm.GetExecInfo(llmID)
	displayName := llmID
	mode := "command"
	promptInput := "args"
	if execInfo != nil {
		if execInfo.DisplayName != "" {
			displayName = execInfo.DisplayName
		}
		mode = execInfo.Mode
		promptInput = execInfo.PromptInput
	}
	r.logger.Infof("Task %d: Calling LLM: %s, mode: %s, prompt: %s, size: %d bytes, timeout: %ds", task.ID, displayName, mode, promptInput, promptSize, timeoutSeconds)
	r.logToProject(project, fmt.Sprintf("Task %d: Calling LLM: %s, mode: %s, prompt: %s, size: %d bytes, timeout: %ds", task.ID, displayName, mode, promptInput, promptSize, timeoutSeconds))

	dispatchReq := &llm.DispatchRequest{
		LLMID:   llmID,
		Prompt:  fullPrompt,
		Options: options,
	}

	r.logger.Infof("Task %d: Dispatching to LLM service", task.ID)
	llmStartTime := time.Now()
	dispatchResult, err := r.llm.Dispatch(dispatchReq)

	// Handle infrastructure errors (command couldn't execute at all)
	if err != nil {
		r.logger.Errorf("Task %d: Infrastructure error: %v", task.ID, err)
		r.logToProject(project, fmt.Sprintf("Task %d: Infrastructure error: %v", task.ID, err))
		r.recordHistoryError(task.UUID, "worker", err.Error(), llmID, task.Work.Invocations)

		// Increment infrastructure retry counter
		task.Work.InfraRetries++
		if task.Work.InfraRetries >= limits.MaxRetries {
			r.logger.Errorf("Task %d: Max infrastructure retries (%d) exceeded", task.ID, limits.MaxRetries)
			r.logToProject(project, fmt.Sprintf("Task %d: Max infrastructure retries exceeded", task.ID))
			r.finishTaskWithInfraError(project, path, task, err.Error(), fullPrompt, result, limits)
		} else {
			// Schedule retry
			r.logger.Infof("Task %d: Will retry (%d/%d infrastructure retries)", task.ID, task.Work.InfraRetries, limits.MaxRetries)
			r.logToProject(project, fmt.Sprintf("Task %d: Will retry (%d/%d infrastructure retries)", task.ID, task.Work.InfraRetries, limits.MaxRetries))
			updates := map[string]interface{}{
				"work": map[string]interface{}{
					"status":        global.ExecutionStatusRetry,
					"error":         err.Error(),
					"infra_retries": task.Work.InfraRetries,
				},
			}
			if _, updateErr := r.tasks.UpdateTask(project, task.UUID, updates); updateErr != nil {
				r.logger.Errorf("Task %d: Failed to save retry status: %v", task.ID, updateErr)
			}
			result.TasksFailed++
		}
		return
	}

	// LLM was invoked - record response with full details
	llmElapsed := time.Since(llmStartTime).Seconds()
	r.logger.Infof("Task %d: LLM exited with code %d, returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, dispatchResult.ResponseSize, llmElapsed)
	r.logToProject(project, fmt.Sprintf("Task %d: LLM exited with code %d, returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, dispatchResult.ResponseSize, llmElapsed))

	// Record response in history with full DispatchResult
	r.recordHistoryResponse(task.UUID, "worker", dispatchResult, llmID, task.Work.Invocations)

	// Check for non-zero exit code (LLM error, not infrastructure)
	if dispatchResult.ExitCode != 0 {
		errorMsg := fmt.Sprintf("LLM exited with code %d", dispatchResult.ExitCode)
		if dispatchResult.Stderr != "" {
			errorMsg += fmt.Sprintf(": %s", dispatchResult.Stderr)
		}
		r.logger.Warnf("Task %d: %s", task.ID, errorMsg)
		r.logToProject(project, fmt.Sprintf("Task %d: %s", task.ID, errorMsg))

		// Check if we're under the invocation limit
		if task.Work.Invocations >= limits.MaxWorker {
			r.logger.Errorf("Task %d: Max worker invocations (%d) exceeded", task.ID, limits.MaxWorker)
			r.finishTask(project, path, task, "", errorMsg, fullPrompt, dispatchResult.Stderr, result, limits)
		} else {
			// Schedule retry
			r.logger.Infof("Task %d: Will retry (%d/%d worker invocations)", task.ID, task.Work.Invocations, limits.MaxWorker)
			r.logToProject(project, fmt.Sprintf("Task %d: Will retry (%d/%d worker invocations)", task.ID, task.Work.Invocations, limits.MaxWorker))
			updates := map[string]interface{}{
				"work": map[string]interface{}{
					"status": global.ExecutionStatusRetry,
					"error":  errorMsg,
				},
			}
			if _, updateErr := r.tasks.UpdateTask(project, task.UUID, updates); updateErr != nil {
				r.logger.Errorf("Task %d: Failed to save retry status: %v", task.ID, updateErr)
			}
			result.TasksFailed++
		}
		return
	}

	// Success - use Stdout as response
	response := dispatchResult.Stdout
	r.logger.Infof("Task %d: Saving result", task.ID)
	r.finishTask(project, path, task, response, "", fullPrompt, dispatchResult.Stderr, result, limits)

	// Check if QA is enabled after successful work completion
	if task.QA.Enabled && task.Work.Status == global.ExecutionStatusDone {
		r.logger.Infof("Task %d: QA enabled, starting QA workflow", task.ID)
		r.executeQAWorkflow(project, path, task, result, timeout, budget, limits)
	}

	// Log final "Finished" status for terminal states only
	// Re-fetch task to get final status after all updates
	finalTask, _, err := r.tasks.GetTask(project, task.UUID)
	if err == nil {
		r.logTaskFinished(project, finalTask)
	}
}

// loadInstructionsFile loads instructions from the appropriate source
func (r *Runner) loadInstructionsFile(project string, task *global.Task) (string, error) {
	source := task.Work.InstructionsFileSource
	if source == "" {
		source = "project" // Default
	}

	var content string
	var err error

	switch source {
	case "project":
		content, err = r.tasks.GetProjectFile(project, task.Work.InstructionsFile)
		if err != nil {
			return "", fmt.Errorf("failed to load instructions file %s from project: %w", task.Work.InstructionsFile, err)
		}

	case "playbook":
		if r.playbooks == nil {
			return "", fmt.Errorf("playbooks service not available")
		}
		// instructions_file should be "playbook-name/path/to/file.md"
		// Parse playbook name and path
		parts := strings.SplitN(task.Work.InstructionsFile, "/", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid playbook instructions_file format (expected 'playbook-name/path'): %s", task.Work.InstructionsFile)
		}
		playbookName := parts[0]
		path := parts[1]

		item, err := r.playbooks.GetFile(playbookName, path, 0, 0)
		if err != nil {
			return "", fmt.Errorf("failed to load instructions file %s from playbook %s: %w", path, playbookName, err)
		}
		content = item.Content

	case "reference":
		if r.reference == nil {
			return "", fmt.Errorf("reference service not available")
		}
		item, err := r.reference.Get(task.Work.InstructionsFile, 0, 0)
		if err != nil {
			return "", fmt.Errorf("failed to load instructions file %s from reference: %w", task.Work.InstructionsFile, err)
		}
		content = item.Content

	default:
		return "", fmt.Errorf("invalid instructions_file_source: %s (must be project, playbook, or reference)", source)
	}

	// Replace <project> placeholders with actual project name (cross-project isolation)
	content = strings.ReplaceAll(content, "<project>", project)
	content = strings.ReplaceAll(content, "\"<project>\"", fmt.Sprintf("\"%s\"", project))

	return content, nil
}

// loadSchemaContent loads schema content from a path.
// The path format determines the source:
// - "playbook-name/path/file.json" -> load from playbook
// - If it starts with '{' -> treat as inline JSON schema
// - Otherwise -> load from project files
func (r *Runner) loadSchemaContent(project, schemaPath string) string {
	if schemaPath == "" {
		return ""
	}

	// Check if it's an inline JSON schema
	if strings.HasPrefix(strings.TrimSpace(schemaPath), "{") {
		return schemaPath
	}

	// Check if it's a playbook path (contains '/')
	if strings.Contains(schemaPath, "/") {
		parts := strings.SplitN(schemaPath, "/", 2)
		if len(parts) == 2 && r.playbooks != nil {
			playbookName := parts[0]
			path := parts[1]
			if item, err := r.playbooks.GetFile(playbookName, path, 0, 0); err == nil {
				return item.Content
			}
			r.logger.Warnf("Failed to load schema from playbook %s/%s", playbookName, path)
		}
	}

	// Try loading from project files
	if content, err := r.tasks.GetProjectFile(project, schemaPath); err == nil {
		return content
	}

	r.logger.Warnf("Failed to load schema from path: %s", schemaPath)
	return ""
}

// templateFileExists checks if a template file exists (may be empty).
// Returns true if the file exists, false if not found.
func (r *Runner) templateFileExists(project, templatePath string) bool {
	if templatePath == "" {
		return false
	}

	// Check if it's a playbook path (contains '/')
	if strings.Contains(templatePath, "/") {
		parts := strings.SplitN(templatePath, "/", 2)
		if len(parts) == 2 && r.playbooks != nil {
			playbookName := parts[0]
			path := parts[1]
			_, err := r.playbooks.GetFile(playbookName, path, 0, 0)
			return err == nil
		}
	}

	// Try loading from project files
	_, err := r.tasks.GetProjectFile(project, templatePath)
	return err == nil
}

// validateTaskSetTemplates validates that all required templates in a task set exist
// Worker templates are REQUIRED. QA templates are required if any task has QA enabled.
// For manifest files (.json), all referenced template files must exist.
// Returns a list of errors for any missing or invalid templates.
func (r *Runner) validateTaskSetTemplates(project string, ts *global.TaskSet) []string {
	var errors []string

	// Worker response template is REQUIRED
	if ts.WorkerResponseTemplate == "" {
		errors = append(errors, "worker_response_template is required but not specified")
	} else {
		content := r.loadSchemaContent(project, ts.WorkerResponseTemplate)
		if content == "" {
			errors = append(errors, fmt.Sprintf("worker_response_template not found: %s", ts.WorkerResponseTemplate))
		}
	}

	// Worker report template is REQUIRED
	if ts.WorkerReportTemplate == "" {
		errors = append(errors, "worker_report_template is required but not specified")
	} else {
		// For manifest files, validate all referenced templates
		errs := r.validateReportTemplate(project, ts.WorkerReportTemplate, "worker_report_template")
		errors = append(errors, errs...)
	}

	// Check if any task has QA enabled
	qaEnabled := false
	for _, task := range ts.Tasks {
		if task.QA.Enabled {
			qaEnabled = true
			break
		}
	}

	// QA templates are required if any task has QA enabled
	if qaEnabled {
		// QA response template is REQUIRED when QA is enabled
		if ts.QAResponseTemplate == "" {
			errors = append(errors, "qa_response_template is required (QA is enabled) but not specified")
		} else {
			content := r.loadSchemaContent(project, ts.QAResponseTemplate)
			if content == "" {
				errors = append(errors, fmt.Sprintf("qa_response_template not found: %s", ts.QAResponseTemplate))
			}
		}

		// QA report template is REQUIRED when QA is enabled
		if ts.QAReportTemplate == "" {
			errors = append(errors, "qa_report_template is required (QA is enabled) but not specified")
		} else {
			// For manifest files, validate all referenced templates
			errs := r.validateReportTemplate(project, ts.QAReportTemplate, "qa_report_template")
			errors = append(errors, errs...)
		}
	}

	return errors
}

// validateReportTemplate validates a report template path (single .md or .json manifest)
// For manifest files, validates that all referenced template files exist
func (r *Runner) validateReportTemplate(project, templatePath, templateName string) []string {
	var errors []string

	// First check if the template path itself exists
	content := r.loadSchemaContent(project, templatePath)
	if content == "" {
		errors = append(errors, fmt.Sprintf("%s not found: %s", templateName, templatePath))
		return errors
	}

	// If it's a manifest file (.json), parse and validate all referenced templates
	if strings.HasSuffix(templatePath, ".json") {
		var configs []global.ReportTemplateConfig
		if err := json.Unmarshal([]byte(content), &configs); err != nil {
			errors = append(errors, fmt.Sprintf("%s manifest parse error: %v", templateName, err))
			return errors
		}

		if len(configs) == 0 {
			errors = append(errors, fmt.Sprintf("%s manifest is empty", templateName))
			return errors
		}

		// Validate each file in the manifest
		manifestDir := filepath.Dir(templatePath)
		for _, config := range configs {
			if config.File == "" {
				errors = append(errors, fmt.Sprintf("%s manifest: entry with suffix '%s' has empty file path", templateName, config.Suffix))
				continue
			}

			// Resolve relative path from manifest location
			var resolvedPath string
			if filepath.IsAbs(config.File) {
				resolvedPath = config.File
			} else {
				resolvedPath = filepath.Join(manifestDir, config.File)
			}

			if !r.templateFileExists(project, resolvedPath) {
				errors = append(errors, fmt.Sprintf("%s manifest: template file not found: %s (suffix: %s)", templateName, resolvedPath, config.Suffix))
			}
		}
	}

	return errors
}

// buildPrompt builds the full prompt from project context, instructions_file, instructions_text, and prompt
func (r *Runner) buildPrompt(project, path string, task *global.Task) (string, error) {
	var sb strings.Builder

	// 0. Always inject project name (mandatory for cross-project isolation)
	sb.WriteString("=== PROJECT CONTEXT ===\n\n")
	sb.WriteString(fmt.Sprintf("Project: %s\n", project))
	sb.WriteString("IMPORTANT: Use this project name for ALL file operations (project_file_list, project_file_get, project_file_search).\n\n")

	// Append optional user-defined context if available
	if proj, err := r.projects.Get(project); err == nil && proj.Context != "" {
		sb.WriteString(proj.Context)
		sb.WriteString("\n\n")
	}

	// 1. Load instructions from file if specified
	if task.Work.InstructionsFile != "" {
		content, err := r.loadInstructionsFile(project, task)
		if err != nil {
			return "", err
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	// 2. Append inline instructions text if specified
	if task.Work.InstructionsText != "" {
		sb.WriteString(task.Work.InstructionsText)
		sb.WriteString("\n\n")
	}

	// 3. Append task-specific prompt with separator
	if task.Work.Prompt != "" {
		sb.WriteString("=== TASK PROMPT ===\n\n")
		sb.WriteString(task.Work.Prompt)
		sb.WriteString("\n\n")
	}

	// 4. Include expected response schema with clear instructions if configured
	if taskSet, err := r.tasks.GetTaskSet(project, path); err == nil && taskSet.WorkerResponseTemplate != "" {
		schema := r.loadSchemaContent(project, taskSet.WorkerResponseTemplate)
		if schema != "" {
			sb.WriteString("=== REQUIRED RESPONSE FORMAT ===\n\n")
			sb.WriteString("IMPORTANT: You MUST respond with a valid JSON object that matches the schema below.\n")
			sb.WriteString("Your response will be validated against this schema. If validation fails, you will be asked to retry.\n\n")
			sb.WriteString("Expected JSON Schema:\n```json\n")
			sb.WriteString(schema)
			sb.WriteString("\n```\n\n")
		}
	}

	// 5. If there was a previous schema error, include it for retry
	if task.Work.Error != "" && task.Work.Invocations > 0 && strings.Contains(task.Work.Error, "schema") {
		sb.WriteString("=== PREVIOUS ATTEMPT FAILED - PLEASE FIX ===\n\n")
		sb.WriteString("Your previous response did not match the required schema. Please review the errors below and provide a corrected response.\n\n")
		sb.WriteString("Validation errors from your previous response:\n")
		sb.WriteString(task.Work.Error)
		sb.WriteString("\n\n")
	}

	return sb.String(), nil
}

// finishTaskWithInfraError marks a task as failed due to infrastructure errors
func (r *Runner) finishTaskWithInfraError(project, path string, task *global.Task, errorMsg, fullPrompt string, result *global.RunResult, limits global.Limits) {
	finalError := fmt.Sprintf("max infrastructure retries exceeded: %s", errorMsg)
	updates := map[string]interface{}{
		"work": map[string]interface{}{
			"status":        global.ExecutionStatusFailed,
			"error":         finalError,
			"infra_retries": task.Work.InfraRetries,
		},
	}
	if _, err := r.tasks.UpdateTask(project, task.UUID, updates); err != nil {
		r.logger.Errorf("Task %d: Failed to save failed status: %v", task.ID, err)
	}

	// Write result file with history for debugging
	r.writeFailedTaskResult(project, task, fullPrompt, "", finalError)

	result.TasksFailed++
}

// finishTask completes a task with success or failure
// llmStderr is optional stderr output from LLM command (pass empty string if not applicable)
func (r *Runner) finishTask(project, path string, task *global.Task, response, errorMsg, fullPrompt, llmStderr string, result *global.RunResult, limits global.Limits) {
	now := time.Now()

	updates := make(map[string]interface{})
	workUpdates := make(map[string]interface{})

	if errorMsg != "" {
		// Failure
		isFinalFailure := task.Work.Invocations >= limits.MaxWorker
		if isFinalFailure {
			workUpdates["status"] = global.ExecutionStatusFailed
			r.logToProject(project, fmt.Sprintf("Task %d: Failed (max worker invocations reached): %s", task.ID, errorMsg))
			r.logger.Errorf("Task %d: Failed (max worker invocations reached): %s", task.ID, errorMsg)
		} else {
			workUpdates["status"] = global.ExecutionStatusWaiting // Allow retry
			r.logToProject(project, fmt.Sprintf("Task %d: Failed, will retry (%d/%d): %s", task.ID, task.Work.Invocations, limits.MaxWorker, errorMsg))
			r.logger.Warnf("Task %d: Failed, will retry (%d/%d): %s", task.ID, task.Work.Invocations, limits.MaxWorker, errorMsg)
		}
		workUpdates["error"] = errorMsg
		updates["work"] = workUpdates
		result.TasksFailed++

		// Save task updates
		if _, err := r.tasks.UpdateTask(project, task.UUID, updates); err != nil {
			r.logger.Errorf("Task %d: Failed to save task status: %v", task.ID, err)
		}

		// Write result file with history for debugging (only on final failure)
		if isFinalFailure {
			r.writeFailedTaskResult(project, task, fullPrompt, response, errorMsg)
		}
	} else {
		// Extract JSON from response (handles markdown code fences and LLM wrapper)
		response = templates.ExtractJSON(response)

		// Validate response against task set schema if configured
		if taskSet, err := r.tasks.GetTaskSet(project, path); err == nil && taskSet.WorkerResponseTemplate != "" {
			schema := r.loadSchemaContent(project, taskSet.WorkerResponseTemplate)
			if schema != "" {
				validationResult, validationErr := r.validator.ValidateJSON([]byte(response), schema)
				if validationErr != nil || (validationResult != nil && !validationResult.Valid) {
					// Build error details
					var errorMessages []string
					var rawErrors []string
					var errorType string

					if validationErr != nil {
						errorType = "parse_error"
						errorMessages = []string{fmt.Sprintf("Failed to parse response: %v", validationErr)}
						rawErrors = errorMessages
					} else {
						errorType = "schema_validation"
						errorMessages = validationResult.Errors
						rawErrors = validationResult.RawErrors
					}

					summary := formatValidationSummary(errorMessages)
					canRetry := task.Work.Invocations < limits.MaxWorker

					// Write error details to file
					errorDetails := &ValidationErrorDetails{
						TaskID:           task.ID,
						TaskUUID:         task.UUID,
						TaskTitle:        task.Title,
						Timestamp:        time.Now(),
						Phase:            "worker",
						ErrorType:        errorType,
						Summary:          summary,
						ValidationErrors: errorMessages,
						RawErrors:        rawErrors,
						LLMResponse:      response,
						LLMStderr:        llmStderr,
						ExpectedSchema:   schema,
						Invocation:       task.Work.Invocations,
						LLMModelID:       task.Work.LLMModelID,
						History:          r.getTaskHistory(task.UUID),
					}
					errorFilename, writeErr := r.writeErrorFile(project, errorDetails)
					if writeErr != nil {
						r.logger.Warnf("Task %d: Failed to write error file: %v", task.ID, writeErr)
						errorFilename = "(failed to write)"
					}

					// Log brief message with file reference
					r.logger.Warnf("Task %d: Worker schema validation failed (%d errors). Details: results/%s", task.ID, len(errorMessages), errorFilename)
					r.logToProject(project, fmt.Sprintf("Task %d: Worker schema validation failed (%d errors). Details: results/%s", task.ID, len(errorMessages), errorFilename))

					// Record in history (without the full schema)
					historyMsg := fmt.Sprintf("Worker schema validation failed:\n- %s", strings.Join(errorMessages, "\n- "))
					r.recordHistory(project, task.UUID, "system", "validation", historyMsg, task.Work.LLMModelID, task.Work.Invocations)

					if canRetry {
						workUpdates["status"] = global.ExecutionStatusWaiting // Allow retry
						r.logToProject(project, fmt.Sprintf("Task %d: Schema validation failed, will retry (%d/%d)", task.ID, task.Work.Invocations, limits.MaxWorker))
						r.logger.Warnf("Task %d: Schema validation failed, will retry (%d/%d)", task.ID, task.Work.Invocations, limits.MaxWorker)
					} else {
						workUpdates["status"] = global.ExecutionStatusFailed
						r.logToProject(project, fmt.Sprintf("Task %d: Schema validation failed, max retries reached", task.ID))
						r.logger.Errorf("Task %d: Schema validation failed, max retries reached (%d/%d)", task.ID, task.Work.Invocations, limits.MaxWorker)
					}
					workUpdates["error"] = historyMsg
					updates["work"] = workUpdates
					result.TasksFailed++

					if _, err := r.tasks.UpdateTask(project, task.UUID, updates); err != nil {
						r.logger.Errorf("Task %d: Failed to save task status: %v", task.ID, err)
					}

					// Write result file with history for final failures
					if !canRetry {
						r.writeFailedTaskResult(project, task, fullPrompt, response, historyMsg)
					}
					return
				}
				r.logger.Infof("Task %d: Response validated against schema", task.ID)
			}
		}

		// Success
		task.Work.Status = global.ExecutionStatusDone // Update local status for QA check
		workUpdates["error"] = ""

		// Only persist 'done' status if QA is NOT enabled
		// If QA is enabled, status stays 'waiting' until QA completes
		if !task.QA.Enabled {
			workUpdates["status"] = global.ExecutionStatusDone
		}
		// Note: if QA enabled, status remains 'waiting' - will be set to 'done' after QA completes

		responseSize := len(response)
		r.logToProject(project, fmt.Sprintf("Task %d: Worker completed successfully (response: %d bytes)", task.ID, responseSize))
		r.logger.Infof("Task %d: Worker completed successfully (response: %d bytes)", task.ID, responseSize)

		// Save result to file with complete audit trail
		taskResult := global.TaskResult{
			TaskID:      task.ID,
			TaskUUID:    task.UUID,
			TaskTitle:   task.Title,
			TaskType:    task.Type,
			CreatedAt:   task.CreatedAt,
			CompletedAt: now,
			Worker: global.WorkerResult{
				InstructionsFile:       task.Work.InstructionsFile,
				InstructionsFileSource: task.Work.InstructionsFileSource,
				InstructionsText:       task.Work.InstructionsText,
				TaskPrompt:             task.Work.Prompt,
				FullPrompt:             fullPrompt,
				Response:               response,
				LLMModelID:             task.Work.LLMModelID,
				Invocations:            task.Work.Invocations,
				Status:                 global.ExecutionStatusDone,
			},
			History: r.getTaskHistory(task.UUID),
		}

		// Save individual result file
		resultsDir := r.tasks.GetResultsDir(project)
		if err := os.MkdirAll(resultsDir, 0755); err != nil {
			r.logger.Warnf("Task %d: Failed to create results directory: %v", task.ID, err)
		} else {
			resultFilename := task.UUID + ".json"
			resultPath := filepath.Join(resultsDir, resultFilename)
			resultData, err := json.MarshalIndent(taskResult, "", "  ")
			if err == nil {
				if writeErr := os.WriteFile(resultPath, resultData, 0644); writeErr != nil {
					r.logger.Warnf("Task %d: Failed to save result file: %v", task.ID, writeErr)
				} else {
					r.logger.Infof("Task %d: Results written to %s (%d bytes)", task.ID, resultFilename, len(resultData))
					r.logToProject(project, fmt.Sprintf("Task %d: Results written to %s", task.ID, resultFilename))
				}
			} else {
				r.logger.Warnf("Task %d: Failed to marshal result: %v", task.ID, err)
			}
		}

		// Note: result is stored in results/<uuid>.json, not in tasks.json
		updates["work"] = workUpdates
		result.TasksSucceeded++

		// Save task updates
		if _, err := r.tasks.UpdateTask(project, task.UUID, updates); err != nil {
			r.logger.Errorf("Task %d: Failed to save task status: %v", task.ID, err)
		}
	}
}

// writeFailedTaskResult writes a result file for a failed task, preserving history for debugging
func (r *Runner) writeFailedTaskResult(project string, task *global.Task, fullPrompt, response, errorMsg string) {
	now := time.Now()

	taskResult := global.TaskResult{
		TaskID:      task.ID,
		TaskUUID:    task.UUID,
		TaskTitle:   task.Title,
		TaskType:    task.Type,
		CreatedAt:   task.CreatedAt,
		CompletedAt: now,
		Worker: global.WorkerResult{
			InstructionsFile:       task.Work.InstructionsFile,
			InstructionsFileSource: task.Work.InstructionsFileSource,
			InstructionsText:       task.Work.InstructionsText,
			TaskPrompt:             task.Work.Prompt,
			FullPrompt:             fullPrompt,
			Response:               response,
			LLMModelID:             task.Work.LLMModelID,
			Invocations:            task.Work.Invocations,
			Status:                 global.ExecutionStatusFailed,
			Error:                  errorMsg,
		},
		History: r.getTaskHistory(task.UUID),
	}

	resultsDir := r.tasks.GetResultsDir(project)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		r.logger.Warnf("Task %d: Failed to create results directory: %v", task.ID, err)
		return
	}

	resultFilename := task.UUID + ".json"
	resultPath := filepath.Join(resultsDir, resultFilename)
	resultData, err := json.MarshalIndent(taskResult, "", "  ")
	if err != nil {
		r.logger.Warnf("Task %d: Failed to marshal failed result: %v", task.ID, err)
		return
	}

	if writeErr := os.WriteFile(resultPath, resultData, 0644); writeErr != nil {
		r.logger.Warnf("Task %d: Failed to save failed result file: %v", task.ID, writeErr)
	} else {
		r.logger.Infof("Task %d: Failed task results written to %s (%d bytes)", task.ID, resultFilename, len(resultData))
		r.logToProject(project, fmt.Sprintf("Task %d: Failed task results written to %s", task.ID, resultFilename))
	}
}

// GetResults retrieves task results
func (r *Runner) GetResults(req *global.ResultsRequest) (*global.ResultsResponse, error) {
	if !r.tasks.ProjectExists(req.Project) {
		return nil, fmt.Errorf("project not found: %s", req.Project)
	}

	// Compile regex patterns if provided
	var workerRegex, qaRegex *regexp.Regexp
	var err error
	if req.WorkerPattern != "" {
		workerRegex, err = regexp.Compile(req.WorkerPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid worker_pattern regex: %w", err)
		}
	}
	if req.QAPattern != "" {
		qaRegex, err = regexp.Compile(req.QAPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid qa_pattern regex: %w", err)
		}
	}

	// If a specific task ID is requested, return single result
	if req.TaskID != nil {
		// Search for the task by ID across all task sets
		taskSetList, err := r.tasks.ListTaskSets(req.Project, req.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to list task sets: %w", err)
		}

		for _, taskSet := range taskSetList.TaskSets {
			for _, task := range taskSet.Tasks {
				if task.ID == *req.TaskID {
					// Check status filter
					if req.Status != "" && task.Work.Status != req.Status {
						return &global.ResultsResponse{
							Project:       req.Project,
							Path:          req.Path,
							TotalCount:    0,
							ReturnedCount: 0,
							Results:       []global.TaskResult{},
						}, nil
					}

					// Load result file if task is done
					if task.Work.Status == global.ExecutionStatusDone {
						resultPath := filepath.Join(r.tasks.GetResultsDir(req.Project), task.UUID+".json")
						data, err := os.ReadFile(resultPath)
						if err != nil {
							return nil, fmt.Errorf("failed to read result file: %w", err)
						}

						var taskResult global.TaskResult
						if err := json.Unmarshal(data, &taskResult); err != nil {
							return nil, fmt.Errorf("failed to parse result file: %w", err)
						}

						// Apply regex filter (OR logic)
						if !r.matchesPatterns(taskResult, workerRegex, qaRegex) {
							return &global.ResultsResponse{
								Project:       req.Project,
								Path:          req.Path,
								TotalCount:    0,
								ReturnedCount: 0,
								Results:       []global.TaskResult{},
							}, nil
						}

						// Return summary or full result
						if req.Summary {
							return &global.ResultsResponse{
								Project:       req.Project,
								Path:          req.Path,
								TotalCount:    1,
								ReturnedCount: 1,
								Summaries: []global.TaskResultSummary{{
									TaskID:     taskResult.TaskID,
									TaskUUID:   taskResult.TaskUUID,
									TaskTitle:  taskResult.TaskTitle,
									WorkStatus: taskResult.Worker.Status,
								}},
							}, nil
						}

						return &global.ResultsResponse{
							Project:       req.Project,
							Path:          req.Path,
							TotalCount:    1,
							ReturnedCount: 1,
							Results:       []global.TaskResult{taskResult},
						}, nil
					}

					// Task exists but not completed
					return &global.ResultsResponse{
						Project:       req.Project,
						Path:          req.Path,
						TotalCount:    0,
						ReturnedCount: 0,
						Results:       []global.TaskResult{},
					}, nil
				}
			}
		}

		return nil, fmt.Errorf("task not found: id=%d", *req.TaskID)
	}

	// List task sets at path
	taskSetList, err := r.tasks.ListTaskSets(req.Project, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to list task sets: %w", err)
	}

	// Collect all completed tasks
	allResults := make([]global.TaskResult, 0)
	resultsDir := r.tasks.GetResultsDir(req.Project)

	for _, taskSet := range taskSetList.TaskSets {
		for _, task := range taskSet.Tasks {
			// Apply status filter
			if req.Status != "" && task.Work.Status != req.Status {
				continue
			}

			// Only include completed tasks
			if task.Work.Status == global.ExecutionStatusDone {
				resultPath := filepath.Join(resultsDir, task.UUID+".json")
				data, err := os.ReadFile(resultPath)
				if err != nil {
					r.logger.Warnf("Failed to read result file for task %s: %v", task.UUID, err)
					continue
				}

				var taskResult global.TaskResult
				if err := json.Unmarshal(data, &taskResult); err != nil {
					r.logger.Warnf("Failed to parse result file for task %s: %v", task.UUID, err)
					continue
				}

				// Apply regex filter (OR logic)
				if !r.matchesPatterns(taskResult, workerRegex, qaRegex) {
					continue
				}

				allResults = append(allResults, taskResult)
			}
		}
	}

	// Apply pagination
	total := len(allResults)
	offset := req.Offset
	limit := req.Limit

	if limit <= 0 {
		limit = global.DefaultLimit
	}

	if offset >= total {
		allResults = []global.TaskResult{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		allResults = allResults[offset:end]
	}

	// Return summary or full results
	if req.Summary {
		summaries := make([]global.TaskResultSummary, len(allResults))
		for i, result := range allResults {
			summaries[i] = global.TaskResultSummary{
				TaskID:     result.TaskID,
				TaskUUID:   result.TaskUUID,
				TaskTitle:  result.TaskTitle,
				WorkStatus: result.Worker.Status,
			}
		}
		return &global.ResultsResponse{
			Project:       req.Project,
			Path:          req.Path,
			TotalCount:    total,
			ReturnedCount: len(summaries),
			Offset:        offset,
			Summaries:     summaries,
		}, nil
	}

	return &global.ResultsResponse{
		Project:       req.Project,
		Path:          req.Path,
		TotalCount:    total,
		ReturnedCount: len(allResults),
		Offset:        offset,
		Results:       allResults,
	}, nil
}

// matchesPatterns checks if a task result matches the provided regex patterns.
// Uses OR logic: if both patterns are provided, task matches if either matches.
// If no patterns provided, returns true.
func (r *Runner) matchesPatterns(result global.TaskResult, workerRegex, qaRegex *regexp.Regexp) bool {
	// No patterns means no filtering
	if workerRegex == nil && qaRegex == nil {
		return true
	}

	// OR logic: match if either pattern matches
	if workerRegex != nil && workerRegex.MatchString(result.Worker.Response) {
		return true
	}
	if qaRegex != nil && result.QA != nil && qaRegex.MatchString(result.QA.Response) {
		return true
	}

	// If patterns were provided but none matched
	return false
}

// ValidateTaskInstructions validates task instructions without loading files
func ValidateTaskInstructions(instructionsFile, instructionsFileSource string) error {
	if instructionsFile == "" {
		return nil // No instructions file specified
	}

	source := instructionsFileSource
	if source == "" {
		source = "project" // Default
	}

	switch source {
	case "project":
		// Project files can be any relative path
		return nil

	case "playbook":
		// Must be in format "playbook-name/path"
		parts := strings.SplitN(instructionsFile, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid playbook instructions_file format - must be 'playbook-name/path/to/file', got: %s", instructionsFile)
		}
		return nil

	case "reference":
		// Reference files can be any path
		return nil

	default:
		return fmt.Errorf("invalid instructions_file_source: %s (must be project, playbook, or reference)", source)
	}
}

// executeQAWorkflow executes the QA workflow after successful work completion
func (r *Runner) executeQAWorkflow(project, path string, task *global.Task, result *global.RunResult, timeout int, budget *runBudget, limits global.Limits) {
	r.logger.Infof("Task %d: Starting QA workflow (invocations: %d, max: %d)", task.ID, task.QA.Invocations, limits.MaxQA)
	r.logToProject(project, fmt.Sprintf("Task %d: Starting QA workflow", task.ID))

	for task.QA.Invocations < limits.MaxQA {
		// Check budget before QA call
		if budget != nil && budget.exceeded {
			r.logger.Warnf("Task %d: LLM budget exceeded, stopping QA workflow", task.ID)
			r.logToProject(project, fmt.Sprintf("Task %d: LLM budget exceeded, QA stopped", task.ID))
			return
		}

		// Execute QA
		err := r.executeQA(project, path, task, budget, limits)
		if err != nil {
			// Check if it's a schema validation error that can be retried
			if sve, ok := IsSchemaValidationError(err); ok {
				if sve.CanRetry {
					r.logger.Infof("Task %d: QA schema validation failed, will retry (%d/%d). Error file: results/%s",
						task.ID, task.QA.Invocations, limits.MaxQA, sve.ErrorFilename)
					r.logToProject(project, fmt.Sprintf("Task %d: QA schema validation failed, will retry (%d/%d)",
						task.ID, task.QA.Invocations, limits.MaxQA))
					// Continue the loop to retry
					continue
				}
				// Max retries reached - status already set to failed in executeQA
				r.logger.Errorf("Task %d: QA schema validation failed, max retries reached (%d/%d)",
					task.ID, task.QA.Invocations, limits.MaxQA)
				r.logToProject(project, fmt.Sprintf("Task %d: QA schema validation failed, max retries reached",
					task.ID))
				return
			}

			// Other errors - mark as failed
			r.logger.Errorf("Task %d: QA execution failed: %v", task.ID, err)
			r.logToProject(project, fmt.Sprintf("Task %d: QA execution failed: %v", task.ID, err))

			// Mark both QA and Work as failed to prevent infinite retry rounds
			qaUpdates := map[string]interface{}{
				"work": map[string]interface{}{
					"status": global.ExecutionStatusFailed,
				},
				"qa": map[string]interface{}{
					"status": global.ExecutionStatusFailed,
				},
			}
			if _, updateErr := r.tasks.UpdateTask(project, task.UUID, qaUpdates); updateErr != nil {
				r.logger.Errorf("Task %d: Failed to save QA failure status: %v", task.ID, updateErr)
			}
			return
		}

		// Handle QA verdict
		switch task.QA.Verdict {
		case global.QAVerdictPass:
			r.logger.Infof("Task %d: QA passed", task.ID)
			r.logToProject(project, fmt.Sprintf("Task %d: QA passed", task.ID))
			return

		case global.QAVerdictEscalate:
			r.logger.Warnf("Task %d: QA escalated - cannot be resolved by QA", task.ID)
			r.logToProject(project, fmt.Sprintf("Task %d: QA escalated", task.ID))
			// Status is already set to "done" with verdict "escalate" - no further action needed
			return

		case global.QAVerdictFail:
			// Check if we can retry
			if task.QA.Invocations >= limits.MaxQA {
				r.logger.Warnf("Task %d: QA failed and max QA invocations reached (%d/%d)", task.ID, task.QA.Invocations, limits.MaxQA)
				r.logToProject(project, fmt.Sprintf("Task %d: QA failed, max invocations reached", task.ID))

				// Mark both QA and Work as failed to prevent infinite retry rounds
				qaUpdates := map[string]interface{}{
					"work": map[string]interface{}{
						"status": global.ExecutionStatusFailed,
					},
					"qa": map[string]interface{}{
						"status": global.ExecutionStatusFailed,
					},
				}
				if _, updateErr := r.tasks.UpdateTask(project, task.UUID, qaUpdates); updateErr != nil {
					r.logger.Errorf("Task %d: Failed to save QA failure status: %v", task.ID, updateErr)
				}
				return
			}

			// Check budget before revision
			if budget != nil && budget.exceeded {
				r.logger.Warnf("Task %d: LLM budget exceeded, stopping QA workflow", task.ID)
				r.logToProject(project, fmt.Sprintf("Task %d: LLM budget exceeded, revision stopped", task.ID))
				return
			}

			// Revise work with QA feedback
			r.logger.Infof("Task %d: QA verdict 'fail', revising work (%d/%d)", task.ID, task.QA.Invocations, limits.MaxQA)
			r.logToProject(project, fmt.Sprintf("Task %d: QA failed, revising work (%d/%d)", task.ID, task.QA.Invocations, limits.MaxQA))

			err = r.reviseWork(project, path, task, timeout, budget, limits)
			if err != nil {
				r.logger.Errorf("Task %d: Work revision failed: %v", task.ID, err)
				r.logToProject(project, fmt.Sprintf("Task %d: Work revision failed: %v", task.ID, err))

				// Mark both QA and Work as failed to prevent infinite retry rounds
				qaUpdates := map[string]interface{}{
					"work": map[string]interface{}{
						"status": global.ExecutionStatusFailed,
					},
					"qa": map[string]interface{}{
						"status":      global.ExecutionStatusFailed,
						"invocations": task.QA.Invocations,
					},
				}
				if _, updateErr := r.tasks.UpdateTask(project, task.UUID, qaUpdates); updateErr != nil {
					r.logger.Errorf("Task %d: Failed to save QA failure status: %v", task.ID, updateErr)
				}
				return
			}
		}
	}
}

// executeQA executes the QA step for a task
func (r *Runner) executeQA(project, path string, task *global.Task, budget *runBudget, limits global.Limits) error {
	r.logger.Infof("Task %d: Executing QA", task.ID)

	// Increment QA invocation count
	task.QA.Invocations++

	// Build QA prompt
	qaPrompt, err := r.buildQAPrompt(project, path, task)
	if err != nil {
		return fmt.Errorf("failed to build QA prompt: %w", err)
	}

	qaPromptSize := len(qaPrompt)
	r.logger.Infof("Task %d: QA prompt built (%d bytes)", task.ID, qaPromptSize)

	// Determine QA LLM
	qaLLMID := task.QA.LLMModelID
	if qaLLMID == "" || qaLLMID == "default" {
		// Use configured default LLM
		defaultLLM := r.config.DefaultLLM()
		if defaultLLM != "" {
			qaLLMID = defaultLLM
		} else {
			// Fallback to first enabled LLM
			enabledLLMs := r.config.EnabledLLMs()
			if len(enabledLLMs) > 0 {
				qaLLMID = enabledLLMs[0].ID
			} else {
				return fmt.Errorf("no LLMs are enabled")
			}
		}
	}

	// Get exec info for detailed logging
	qaExecInfo := r.llm.GetExecInfo(qaLLMID)
	qaDisplayName := qaLLMID
	qaMode := "command"
	qaPromptInput := "args"
	if qaExecInfo != nil {
		if qaExecInfo.DisplayName != "" {
			qaDisplayName = qaExecInfo.DisplayName
		}
		qaMode = qaExecInfo.Mode
		qaPromptInput = qaExecInfo.PromptInput
	}
	r.logger.Infof("Task %d: Calling QA LLM: %s, mode: %s, prompt: %s, size: %d bytes", task.ID, qaDisplayName, qaMode, qaPromptInput, qaPromptSize)
	r.logToProject(project, fmt.Sprintf("Task %d: Calling QA LLM: %s, mode: %s, prompt: %s, size: %d bytes", task.ID, qaDisplayName, qaMode, qaPromptInput, qaPromptSize))

	// Record QA prompt in history
	r.recordHistory(project, task.UUID, "qa", "prompt", qaPrompt, qaLLMID, task.QA.Invocations)

	// Update QA status to processing with invocation count
	qaUpdates := map[string]interface{}{
		"qa": map[string]interface{}{
			"status":      global.ExecutionStatusProcessing,
			"invocations": task.QA.Invocations,
		},
	}
	if _, err := r.tasks.UpdateTask(project, task.UUID, qaUpdates); err != nil {
		r.logger.Warnf("Task %d: Failed to save QA processing status: %v", task.ID, err)
	}

	// Check budget before LLM call
	if !budget.checkAndIncrement() {
		return fmt.Errorf("LLM budget exceeded")
	}

	// Call LLM
	dispatchReq := &llm.DispatchRequest{
		LLMID:  qaLLMID,
		Prompt: qaPrompt,
	}

	qaLLMStartTime := time.Now()
	dispatchResult, err := r.llm.Dispatch(dispatchReq)
	if err != nil {
		r.recordHistory(project, task.UUID, "system", "error", fmt.Sprintf("QA LLM call failed: %v", err), qaLLMID, task.QA.Invocations)
		return fmt.Errorf("QA LLM call failed: %w", err)
	}

	// Extract response as string
	qaResponse := ""
	if dispatchResult.Output != nil {
		switch v := dispatchResult.Output.(type) {
		case string:
			qaResponse = v
		default:
			// Try to marshal to JSON
			if data, err := json.Marshal(v); err == nil {
				qaResponse = string(data)
			} else {
				r.logger.Warnf("Task %d: Failed to marshal QA response: %v", task.ID, err)
			}
		}
	}

	qaLLMElapsed := time.Since(qaLLMStartTime).Seconds()
	r.logger.Infof("Task %d: QA LLM exited with code %d and returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, len(qaResponse), qaLLMElapsed)
	r.logToProject(project, fmt.Sprintf("Task %d: QA LLM exited with code %d and returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, len(qaResponse), qaLLMElapsed))

	// Record QA response in history with full DispatchResult (raw response before JSON extraction)
	r.recordHistoryResponse(task.UUID, "qa", dispatchResult, qaLLMID, task.QA.Invocations)

	// Extract JSON from response (handles markdown code fences and LLM wrapper)
	qaResponse = templates.ExtractJSON(qaResponse)

	// Validate QA response against task set schema if configured
	if taskSet, err := r.tasks.GetTaskSet(project, path); err == nil && taskSet.QAResponseTemplate != "" {
		schema := r.loadSchemaContent(project, taskSet.QAResponseTemplate)
		if schema != "" {
			validationResult, validationErr := r.validator.ValidateJSON([]byte(qaResponse), schema)
			if validationErr != nil || (validationResult != nil && !validationResult.Valid) {
				// Build error details
				var errorMessages []string
				var rawErrors []string
				var errorType string

				if validationErr != nil {
					errorType = "parse_error"
					errorMessages = []string{fmt.Sprintf("Failed to parse response: %v", validationErr)}
					rawErrors = errorMessages
				} else {
					errorType = "schema_validation"
					errorMessages = validationResult.Errors
					rawErrors = validationResult.RawErrors
				}

				summary := formatValidationSummary(errorMessages)
				canRetry := task.QA.Invocations < limits.MaxQA

				// Write error details to file
				errorDetails := &ValidationErrorDetails{
					TaskID:           task.ID,
					TaskUUID:         task.UUID,
					TaskTitle:        task.Title,
					Timestamp:        time.Now(),
					Phase:            "qa",
					ErrorType:        errorType,
					Summary:          summary,
					ValidationErrors: errorMessages,
					RawErrors:        rawErrors,
					LLMResponse:      qaResponse,
					LLMStderr:        dispatchResult.Stderr,
					ExpectedSchema:   schema,
					Invocation:       task.QA.Invocations,
					LLMModelID:       qaLLMID,
					History:          r.getTaskHistory(task.UUID),
				}
				errorFilename, writeErr := r.writeErrorFile(project, errorDetails)
				if writeErr != nil {
					r.logger.Warnf("Task %d: Failed to write error file: %v", task.ID, writeErr)
					errorFilename = "(failed to write)"
				}

				// Log brief message with file reference
				r.logger.Warnf("Task %d: QA schema validation failed (%d errors). Details: results/%s", task.ID, len(errorMessages), errorFilename)
				r.logToProject(project, fmt.Sprintf("Task %d: QA schema validation failed (%d errors). Details: results/%s", task.ID, len(errorMessages), errorFilename))

				// Record in history (without the full schema)
				historyMsg := fmt.Sprintf("QA schema validation failed:\n- %s", strings.Join(errorMessages, "\n- "))
				r.recordHistory(project, task.UUID, "system", "validation", historyMsg, qaLLMID, task.QA.Invocations)

				// Save QA response and error for audit trail and retry prompt
				qaUpdates = map[string]interface{}{
					"qa": map[string]interface{}{
						"result":      qaResponse,
						"error":       historyMsg,
						"invocations": task.QA.Invocations,
					},
				}

				// Set status based on retry capability
				if canRetry {
					qaUpdates["qa"].(map[string]interface{})["status"] = global.ExecutionStatusWaiting
				} else {
					qaUpdates["qa"].(map[string]interface{})["status"] = global.ExecutionStatusFailed
					// Also mark work as failed to prevent infinite retry rounds
					qaUpdates["work"] = map[string]interface{}{
						"status": global.ExecutionStatusFailed,
					}
				}

				if _, updateErr := r.tasks.UpdateTask(project, task.UUID, qaUpdates); updateErr != nil {
					r.logger.Warnf("Task %d: Failed to save QA error state: %v", task.ID, updateErr)
				}

				return &SchemaValidationError{
					Phase:            "qa",
					ValidationErrors: errorMessages,
					ErrorFilename:    errorFilename,
					CanRetry:         canRetry,
				}
			}
			r.logger.Infof("Task %d: QA response validated against schema", task.ID)
		}
	}

	// Parse QA response to extract verdict
	qaResult, err := r.validator.ParseQAResponse([]byte(qaResponse))
	if err != nil {
		return fmt.Errorf("failed to parse QA response: %w", err)
	}

	r.logger.Infof("Task %d: QA response parsed (verdict: %s)", task.ID, qaResult.Verdict)

	// Update task with QA results AND set work.status to done
	// This is the final status update - task is now fully complete
	qaUpdates = map[string]interface{}{
		"work": map[string]interface{}{
			"status": global.ExecutionStatusDone, // Task fully complete after QA
		},
		"qa": map[string]interface{}{
			"status":      global.ExecutionStatusDone,
			"result":      qaResponse,
			"verdict":     qaResult.Verdict,
			"invocations": task.QA.Invocations,
		},
	}

	updatedTask, err := r.tasks.UpdateTask(project, task.UUID, qaUpdates)
	if err != nil {
		return fmt.Errorf("failed to save QA results: %w", err)
	}

	// Update local task reference
	task.QA = updatedTask.QA

	// Update result file with QA data
	resultsDir := r.tasks.GetResultsDir(project)
	resultFilename := task.UUID + ".json"
	resultPath := filepath.Join(resultsDir, resultFilename)

	// Load existing result
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		r.logger.Warnf("Task %d: Failed to read result file for QA update: %v", task.ID, err)
	} else {
		var taskResult global.TaskResult
		if err := json.Unmarshal(resultData, &taskResult); err != nil {
			r.logger.Warnf("Task %d: Failed to parse result file for QA update: %v", task.ID, err)
		} else {
			// Add QA result
			taskResult.QA = &global.QAResult{
				InstructionsFile:       task.QA.InstructionsFile,
				InstructionsFileSource: task.QA.InstructionsFileSource,
				InstructionsText:       task.QA.InstructionsText,
				FullPrompt:             qaPrompt,
				Response:               qaResponse,
				Verdict:                qaResult.Verdict,
				LLMModelID:             qaLLMID,
				Invocations:            task.QA.Invocations,
				Status:                 global.ExecutionStatusDone,
			}

			// Update history with latest messages
			taskResult.History = r.getTaskHistory(task.UUID)

			// Save updated result
			updatedData, err := json.MarshalIndent(taskResult, "", "  ")
			if err == nil {
				if writeErr := os.WriteFile(resultPath, updatedData, 0644); writeErr != nil {
					r.logger.Warnf("Task %d: Failed to save QA result to file: %v", task.ID, writeErr)
				} else {
					r.logger.Infof("Task %d: QA results written to %s (%d bytes)", task.ID, resultFilename, len(updatedData))
					r.logToProject(project, fmt.Sprintf("Task %d: QA results written to %s", task.ID, resultFilename))
				}
			} else {
				r.logger.Warnf("Task %d: Failed to marshal QA result: %v", task.ID, err)
			}
		}
	}

	return nil
}

// buildQAPrompt builds the QA prompt from project context, instructions and work result
func (r *Runner) buildQAPrompt(project, path string, task *global.Task) (string, error) {
	var sb strings.Builder

	// 0. Always inject project name (mandatory for cross-project isolation)
	sb.WriteString("=== PROJECT CONTEXT ===\n\n")
	sb.WriteString(fmt.Sprintf("Project: %s\n", project))
	sb.WriteString("IMPORTANT: Use this project name for ALL file operations (project_file_list, project_file_get, project_file_search).\n\n")

	// Append optional user-defined context if available
	if proj, err := r.projects.Get(project); err == nil && proj.Context != "" {
		sb.WriteString(proj.Context)
		sb.WriteString("\n\n")
	}

	// 1. Load instructions from file if specified
	if task.QA.InstructionsFile != "" {
		// Temporarily use QA's instructions for loading
		originalFile := task.Work.InstructionsFile
		originalSource := task.Work.InstructionsFileSource

		task.Work.InstructionsFile = task.QA.InstructionsFile
		task.Work.InstructionsFileSource = task.QA.InstructionsFileSource

		content, err := r.loadInstructionsFile(project, task)

		// Restore original values
		task.Work.InstructionsFile = originalFile
		task.Work.InstructionsFileSource = originalSource

		if err != nil {
			return "", err
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	// 2. Append inline instructions text if specified
	if task.QA.InstructionsText != "" {
		sb.WriteString(task.QA.InstructionsText)
		sb.WriteString("\n\n")
	}

	// 3. Append QA-specific prompt with separator
	if task.QA.Prompt != "" {
		sb.WriteString("=== QA TASK PROMPT ===\n\n")
		sb.WriteString(task.QA.Prompt)
		sb.WriteString("\n\n")
	}

	// 3.5. Include expected response schema with clear instructions
	if taskSet, err := r.tasks.GetTaskSet(project, path); err == nil && taskSet.QAResponseTemplate != "" {
		schema := r.loadSchemaContent(project, taskSet.QAResponseTemplate)
		if schema != "" {
			sb.WriteString("=== REQUIRED RESPONSE FORMAT ===\n\n")
			sb.WriteString("IMPORTANT: You MUST respond with a valid JSON object that matches the schema below.\n")
			sb.WriteString("Your response will be validated against this schema. If validation fails, you will be asked to retry.\n\n")
			sb.WriteString("CRITICAL: Your JSON response MUST include a 'verdict' field with one of these exact values:\n")
			sb.WriteString("  - \"pass\" - The work meets all requirements\n")
			sb.WriteString("  - \"fail\" - The work has critical issues that cannot be resolved\n")
			sb.WriteString("  - \"escalate\" - The work needs revision and should be sent back to the worker\n\n")
			sb.WriteString("Expected JSON Schema:\n```json\n")
			sb.WriteString(schema)
			sb.WriteString("\n```\n\n")
		}
	}

	// 3.6. If there was a previous schema error, include it for retry
	if task.QA.Error != "" && task.QA.Invocations > 0 {
		sb.WriteString("=== PREVIOUS ATTEMPT FAILED - PLEASE FIX ===\n\n")
		sb.WriteString("Your previous response did not match the required schema. Please review the errors below and provide a corrected response.\n\n")
		sb.WriteString("Common mistakes to avoid:\n")
		sb.WriteString("  - Using 'passed: true/false' instead of 'verdict: \"pass\"/\"fail\"'\n")
		sb.WriteString("  - Using 'qa_verdict' instead of 'verdict'\n")
		sb.WriteString("  - Missing the 'verdict' field entirely\n")
		sb.WriteString("  - Using boolean values where strings are expected\n\n")
		sb.WriteString("Validation errors from your previous response:\n")
		sb.WriteString(task.QA.Error)
		sb.WriteString("\n\n")
	}

	// 4. Append work result for QA to review (load full result from results file)
	sb.WriteString("=== WORK RESULT TO REVIEW ===\n\n")

	// Load full result from results file
	var fullResult string
	resultsDir := r.tasks.GetResultsDir(project)
	resultPath := filepath.Join(resultsDir, task.UUID+".json")
	if data, err := os.ReadFile(resultPath); err == nil {
		var taskResult global.TaskResult
		if err := json.Unmarshal(data, &taskResult); err == nil {
			fullResult = taskResult.Worker.Response
			r.logger.Infof("Task %d: Loaded full result for QA (%d bytes)", task.ID, len(fullResult))
		}
	}

	if fullResult == "" {
		return "", fmt.Errorf("work result not found in results file")
	}

	sb.WriteString(fullResult)

	return sb.String(), nil
}

// reviseWork re-executes the work with QA feedback
func (r *Runner) reviseWork(project, path string, task *global.Task, timeout int, budget *runBudget, limits global.Limits) error {
	r.logger.Infof("Task %d: Revising work with QA feedback", task.ID)
	r.logToProject(project, fmt.Sprintf("Task %d: Revising work with QA feedback", task.ID))

	// Build revised prompt with QA feedback appended
	var sb strings.Builder

	// 0. Always inject project name (mandatory for cross-project isolation)
	sb.WriteString("=== PROJECT CONTEXT ===\n\n")
	sb.WriteString(fmt.Sprintf("Project: %s\n", project))
	sb.WriteString("IMPORTANT: Use this project name for ALL file operations (project_file_list, project_file_get, project_file_search).\n\n")

	// Append optional user-defined context if available
	if proj, err := r.projects.Get(project); err == nil && proj.Context != "" {
		sb.WriteString(proj.Context)
		sb.WriteString("\n\n")
	}

	// 1. Load instructions from file if specified
	if task.Work.InstructionsFile != "" {
		content, err := r.loadInstructionsFile(project, task)
		if err != nil {
			return fmt.Errorf("failed to load instructions file: %w", err)
		}
		sb.WriteString(content)
		sb.WriteString("\n\n")
	}

	// 2. Append inline instructions text if specified
	if task.Work.InstructionsText != "" {
		sb.WriteString(task.Work.InstructionsText)
		sb.WriteString("\n\n")
	}

	// 3. Append task-specific prompt with separator
	if task.Work.Prompt != "" {
		sb.WriteString("=== TASK PROMPT ===\n\n")
		sb.WriteString(task.Work.Prompt)
		sb.WriteString("\n\n")
	}

	// 4. Include expected response schema with clear instructions if configured
	if taskSet, err := r.tasks.GetTaskSet(project, path); err == nil && taskSet.WorkerResponseTemplate != "" {
		schema := r.loadSchemaContent(project, taskSet.WorkerResponseTemplate)
		if schema != "" {
			sb.WriteString("=== REQUIRED RESPONSE FORMAT ===\n\n")
			sb.WriteString("IMPORTANT: You MUST respond with a valid JSON object that matches the schema below.\n")
			sb.WriteString("Your response will be validated against this schema. If validation fails, you will be asked to retry.\n\n")
			sb.WriteString("Expected JSON Schema:\n```json\n")
			sb.WriteString(schema)
			sb.WriteString("\n```\n\n")
		}
	}

	// 5. Append QA feedback
	// Include the full QA result so the worker can see all feedback details
	sb.WriteString("=== QA FEEDBACK ===\n\n")
	sb.WriteString(fmt.Sprintf("The previous attempt was reviewed by QA and received verdict: %s\n\n", task.QA.Verdict))
	sb.WriteString("Full QA response:\n")

	// Load QA result from results file
	resultsDir := r.tasks.GetResultsDir(project)
	resultPath := filepath.Join(resultsDir, task.UUID+".json")
	if data, err := os.ReadFile(resultPath); err == nil {
		var taskResult global.TaskResult
		if err := json.Unmarshal(data, &taskResult); err == nil && taskResult.QA != nil {
			sb.WriteString(taskResult.QA.Response)
		} else {
			sb.WriteString("(QA response not found in results file)")
		}
	} else {
		sb.WriteString("(Failed to load QA result)")
	}

	fullPrompt := sb.String()
	promptSize := len(fullPrompt)
	r.logger.Infof("Task %d: Revised prompt built (%d bytes)", task.ID, promptSize)

	// Determine LLM
	llmID := task.Work.LLMModelID
	if llmID == "" || llmID == "default" {
		defaultLLM := r.config.DefaultLLM()
		if defaultLLM != "" {
			llmID = defaultLLM
		} else {
			enabledLLMs := r.config.EnabledLLMs()
			if len(enabledLLMs) > 0 {
				llmID = enabledLLMs[0].ID
			} else {
				return fmt.Errorf("no LLMs are enabled")
			}
		}
		// Store resolved LLM ID for result file
		task.Work.LLMModelID = llmID
	}

	// Build dispatch options with timeout if specified
	var options *llm.DispatchOptions
	timeoutSeconds := global.DefaultTimeout
	if timeout > 0 {
		timeoutSeconds = timeout
		options = &llm.DispatchOptions{
			Timeout: timeout,
		}
	}

	// Get exec info for detailed logging
	revExecInfo := r.llm.GetExecInfo(llmID)
	revDisplayName := llmID
	revMode := "command"
	revPromptInput := "args"
	if revExecInfo != nil {
		if revExecInfo.DisplayName != "" {
			revDisplayName = revExecInfo.DisplayName
		}
		revMode = revExecInfo.Mode
		revPromptInput = revExecInfo.PromptInput
	}
	r.logger.Infof("Task %d: Calling revision LLM: %s, mode: %s, prompt: %s, size: %d bytes, timeout: %ds", task.ID, revDisplayName, revMode, revPromptInput, promptSize, timeoutSeconds)
	r.logToProject(project, fmt.Sprintf("Task %d: Calling revision LLM: %s, mode: %s, prompt: %s, size: %d bytes, timeout: %ds", task.ID, revDisplayName, revMode, revPromptInput, promptSize, timeoutSeconds))

	// Update work metadata (keep status as 'waiting' until fully complete)
	now := time.Now()
	task.Work.Invocations++

	// Record revision prompt in history
	r.recordHistory(project, task.UUID, "worker", "prompt", fullPrompt, llmID, task.Work.Invocations)
	workUpdates := map[string]interface{}{
		"work": map[string]interface{}{
			"invocations":     task.Work.Invocations,
			"last_attempt_at": &now,
		},
	}
	if _, err := r.tasks.UpdateTask(project, task.UUID, workUpdates); err != nil {
		r.logger.Warnf("Task %d: Failed to save work metadata: %v", task.ID, err)
	}

	// Check budget before LLM call
	if !budget.checkAndIncrement() {
		return fmt.Errorf("LLM budget exceeded")
	}

	// Call LLM
	dispatchReq := &llm.DispatchRequest{
		LLMID:   llmID,
		Prompt:  fullPrompt,
		Options: options,
	}

	revisionLLMStartTime := time.Now()
	dispatchResult, err := r.llm.Dispatch(dispatchReq)
	if err != nil {
		r.recordHistory(project, task.UUID, "system", "error", fmt.Sprintf("Revision LLM call failed: %v", err), llmID, task.Work.Invocations)
		// Update work with error
		workUpdates = map[string]interface{}{
			"work": map[string]interface{}{
				"status": global.ExecutionStatusFailed,
				"error":  err.Error(),
			},
		}
		if _, updateErr := r.tasks.UpdateTask(project, task.UUID, workUpdates); updateErr != nil {
			r.logger.Errorf("Task %d: Failed to save work error: %v", task.ID, updateErr)
		}
		return fmt.Errorf("LLM call failed: %w", err)
	}

	// Extract response as string
	response := ""
	if dispatchResult.Output != nil {
		switch v := dispatchResult.Output.(type) {
		case string:
			response = v
		default:
			if data, err := json.Marshal(v); err == nil {
				response = string(data)
			} else {
				r.logger.Warnf("Task %d: Failed to marshal response: %v", task.ID, err)
			}
		}
	}

	responseSize := len(response)
	revisionLLMElapsed := time.Since(revisionLLMStartTime).Seconds()
	r.logger.Infof("Task %d: Work revision LLM exited with code %d and returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, responseSize, revisionLLMElapsed)
	r.logToProject(project, fmt.Sprintf("Task %d: Work revision LLM exited with code %d and returned %d bytes in %.1fs", task.ID, dispatchResult.ExitCode, responseSize, revisionLLMElapsed))

	// Record revision response in history with full DispatchResult (raw response before JSON extraction)
	r.recordHistoryResponse(task.UUID, "worker", dispatchResult, llmID, task.Work.Invocations)

	// Extract JSON from response (handles markdown code fences)
	response = templates.ExtractJSON(response)

	// Save revised work result
	resultsDir = r.tasks.GetResultsDir(project)
	taskResult := global.TaskResult{
		TaskID:      task.ID,
		TaskUUID:    task.UUID,
		TaskTitle:   task.Title,
		TaskType:    task.Type,
		CreatedAt:   task.CreatedAt,
		CompletedAt: time.Now(),
		Worker: global.WorkerResult{
			InstructionsFile:       task.Work.InstructionsFile,
			InstructionsFileSource: task.Work.InstructionsFileSource,
			InstructionsText:       task.Work.InstructionsText,
			TaskPrompt:             task.Work.Prompt,
			FullPrompt:             fullPrompt,
			Response:               response,
			LLMModelID:             task.Work.LLMModelID,
			Invocations:            task.Work.Invocations,
			Status:                 global.ExecutionStatusDone,
		},
		History: r.getTaskHistory(task.UUID),
	}

	// Save individual result file
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		r.logger.Warnf("Task %d: Failed to create results directory: %v", task.ID, err)
	} else {
		resultFilename := task.UUID + ".json"
		resultPath := filepath.Join(resultsDir, resultFilename)
		resultData, err := json.MarshalIndent(taskResult, "", "  ")
		if err == nil {
			if writeErr := os.WriteFile(resultPath, resultData, 0644); writeErr != nil {
				r.logger.Warnf("Task %d: Failed to save result file: %v", task.ID, writeErr)
			} else {
				r.logger.Infof("Task %d: Revised results written to %s (%d bytes)", task.ID, resultFilename, len(resultData))
				r.logToProject(project, fmt.Sprintf("Task %d: Revised results written to %s", task.ID, resultFilename))
			}
		} else {
			r.logger.Warnf("Task %d: Failed to marshal result: %v", task.ID, err)
		}
	}

	// Update work with revised result (store full result for template rendering)
	workUpdates = map[string]interface{}{
		"work": map[string]interface{}{
			"status":      global.ExecutionStatusDone,
			"result":      response,
			"error":       "",
			"invocations": task.Work.Invocations,
		},
	}

	updatedTask, err := r.tasks.UpdateTask(project, task.UUID, workUpdates)
	if err != nil {
		return fmt.Errorf("failed to save revised work result: %w", err)
	}

	// Update local task reference
	task.Work = updatedTask.Work

	r.logger.Infof("Task %d: Work revision completed successfully", task.ID)
	r.logToProject(project, fmt.Sprintf("Task %d: Work revision completed", task.ID))

	return nil
}

// GenerateReport (generateAndSaveReport) generates reports after task execution completes.
// Supports multiple reports via JSON manifest files. If a taskset's WorkerReportTemplate
// points to a .json file, it's parsed as a manifest containing multiple {suffix, file} entries.
// Each suffix produces a separate report file (e.g., Report.md, Internal.md, Summary.md).
// GenerateReport generates reports for a project's task results.
// This is the public API for report generation, callable from handlers.
// Returns the list of generated report filenames.
func (r *Runner) GenerateReport(project, pathFilter string) ([]string, error) {
	return r.generateAndSaveReport(project, pathFilter)
}

func (r *Runner) generateAndSaveReport(project, pathFilter string) ([]string, error) {
	r.logger.Infof("Starting report generation for project %s", project)
	r.logToProject(project, "Starting report generation")

	// Check if projects service is available
	if r.projects == nil {
		r.logger.Warnf("Projects service not available, skipping report generation")
		return nil, fmt.Errorf("projects service not available")
	}

	// Get all task sets (optionally filtered by path)
	taskSetList, err := r.tasks.ListTaskSets(project, pathFilter)
	if err != nil {
		r.logger.Errorf("Failed to list task sets for report: %v", err)
		r.logToProject(project, fmt.Sprintf("Auto-report generation failed: %v", err))
		return nil, fmt.Errorf("failed to list task sets: %w", err)
	}

	// Add each taskset to the report manifest (before generating content)
	for _, ts := range taskSetList.TaskSets {
		if _, err := r.projects.AddToManifest(project, ts.Path); err != nil {
			r.logger.Warnf("Failed to add taskset %s to manifest: %v", ts.Path, err)
		}
	}

	// Build the report
	filter := &reporting.ReportFilter{
		PathPrefix: pathFilter,
	}
	resultsDir := r.tasks.GetResultsDir(project)
	report := r.reporter.BuildReport(project, taskSetList.TaskSets, filter, resultsDir)

	// Collect all unique report suffixes and their template configs
	// Map: suffix -> template file path (from first taskset that defines it)
	reportConfigs := make(map[string]string)

	for _, ts := range report.TaskSets {
		configs := r.reporter.LoadTemplateConfigs(ts.WorkerReportTemplate)
		for _, cfg := range configs {
			if _, exists := reportConfigs[cfg.Suffix]; !exists {
				reportConfigs[cfg.Suffix] = cfg.File
			}
		}
	}

	// If no configs found, use default "Report" with no template
	if len(reportConfigs) == 0 {
		reportConfigs["Report"] = ""
	}

	// Generate content for each report suffix
	var generatedReports []string
	prefix, _ := r.projects.GetReportPrefix(project)

	for suffix, templateFile := range reportConfigs {
		var content strings.Builder

		for _, ts := range report.TaskSets {
			// Find the template file for this suffix from this taskset
			tsTemplateFile := templateFile // default from first taskset
			tsConfigs := r.reporter.LoadTemplateConfigs(ts.WorkerReportTemplate)
			for _, cfg := range tsConfigs {
				if cfg.Suffix == suffix {
					tsTemplateFile = cfg.File
					break
				}
			}

			// Write task set header (## level since main report has # header)
			content.WriteString(fmt.Sprintf("## %s\n\n", ts.Title))

			// Write each task - template handles the full output including header
			for _, task := range ts.Tasks {
				if task.WorkResult != "" {
					// Use template if configured, otherwise raw result
					renderedResult := r.reporter.RenderWithTemplate(task, tsTemplateFile)
					trimmedResult := strings.TrimSpace(renderedResult)
					// Only add content and separator if template produced output
					if trimmedResult != "" {
						content.WriteString(trimmedResult)
						content.WriteString("\n\n---\n\n")
					}
				} else {
					// No result yet - just show basic task info
					content.WriteString(fmt.Sprintf("### %s\n\n", task.Title))
					content.WriteString(fmt.Sprintf("**Task**: %d (%s)\n\n---\n\n", task.ID, task.WorkStatus))
				}
			}
		}

		// Determine report name based on suffix
		var reportName string
		if suffix == "Report" {
			reportName = "" // Empty means main report
		} else {
			reportName = suffix
		}

		// Append to report using reports domain
		if err := r.projects.AppendReport(project, content.String(), reportName); err != nil {
			r.logger.Errorf("Failed to append to report %s: %v", suffix, err)
			r.logToProject(project, fmt.Sprintf("Failed to save auto-report %s: %v", suffix, err))
			continue
		}

		filename := prefix + suffix + ".md"
		// Note: projects.AppendReport already logs the write
		r.logToProject(project, fmt.Sprintf("Wrote to report: %s", filename))
		generatedReports = append(generatedReports, filename)
	}

	// Sync the logger to ensure all log entries are flushed before we return
	// This is important for graceful shutdown - we need logs written before exit
	if err := r.logger.Sync(); err != nil {
		r.logger.Warnf("Failed to sync logger: %v", err)
	}

	r.logger.Infof("Report generation complete for project %s: %d report(s) written", project, len(generatedReports))
	r.logToProject(project, fmt.Sprintf("Report generation complete: %d report(s) written", len(generatedReports)))

	return generatedReports, nil
}
