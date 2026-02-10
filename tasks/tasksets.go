/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package tasks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/gofrs/flock"
	"github.com/google/uuid"
)

// Service provides task set and task management operations
type Service struct {
	config   *config.Config
	projects *projects.Service
	logger   *logging.Logger
}

// TaskSetListResult represents the response for task set list operations
type TaskSetListResult struct {
	TaskSets []*global.TaskSet `json:"task_sets"`
	Total    int               `json:"total"`
}

// TaskListResult represents the response for task list operations
type TaskListResult struct {
	Tasks []*global.Task `json:"tasks"`
	Total int            `json:"total"`
	Path  string         `json:"path"`
}

// pathSegmentRegex validates individual path segments
var pathSegmentRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// NewService creates a new task service instance
func NewService(cfg *config.Config, projectsService *projects.Service, logger *logging.Logger) *Service {
	return &Service{
		config:   cfg,
		projects: projectsService,
		logger:   logger,
	}
}

// validatePath validates a task set path
func validatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	segments := strings.Split(path, global.TaskPathSeparator)
	if len(segments) > global.MaxTaskPathDepth {
		return fmt.Errorf("path depth %d exceeds maximum %d", len(segments), global.MaxTaskPathDepth)
	}
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("path contains empty segment")
		}
		if !pathSegmentRegex.MatchString(seg) {
			return fmt.Errorf("invalid path segment '%s': must be lowercase alphanumeric with hyphens/underscores, cannot start with - or _", seg)
		}
	}
	return nil
}

// getTaskSetFilePath returns the path to a task set JSON file
// Path "analysis/code" becomes "analysis-code.json"
func (s *Service) getTaskSetFilePath(project, path string) string {
	filename := strings.ReplaceAll(path, global.TaskPathSeparator, global.ListPathSeparator) + ".json"
	return filepath.Join(s.projects.GetTasksDir(project), filename)
}

// getLockPath returns the lock file path for a task set
func (s *Service) getLockPath(project, path string) string {
	return s.getTaskSetFilePath(project, path) + ".lock"
}

// withLock executes a function with file-level locking
func (s *Service) withLock(project, path string, fn func() error) error {
	lockPath := s.getLockPath(project, path)

	// Ensure directory exists
	dir := filepath.Dir(lockPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	lock := flock.New(lockPath)
	if err := lock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer lock.Unlock()

	return fn()
}

// loadTaskSet loads a task set from disk
func (s *Service) loadTaskSet(project, path string) (*global.TaskSet, error) {
	filePath := s.getTaskSetFilePath(project, path)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task set not found: %s", path)
		}
		return nil, fmt.Errorf("failed to read task set: %w", err)
	}

	var taskSet global.TaskSet
	if err := json.Unmarshal(data, &taskSet); err != nil {
		return nil, fmt.Errorf("failed to parse task set: %w", err)
	}

	if taskSet.Tasks == nil {
		taskSet.Tasks = []global.Task{}
	}

	return &taskSet, nil
}

// saveTaskSet saves a task set to disk with atomic writes
func (s *Service) saveTaskSet(project, path string, taskSet *global.TaskSet) error {
	filePath := s.getTaskSetFilePath(project, path)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Ensure Tasks is not nil
	if taskSet.Tasks == nil {
		taskSet.Tasks = []global.Task{}
	}

	data, err := json.MarshalIndent(taskSet, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task set: %w", err)
	}

	// Atomic write
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// CreateTaskSet creates a new task set at the given path
func (s *Service) CreateTaskSet(project, path, title, description string, templates *global.DefaultTemplates, parallel bool, limits global.Limits) (*global.TaskSet, error) {
	// Validate inputs
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if title == "" {
		return nil, fmt.Errorf("title cannot be empty")
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	// Apply defaults to limits (zeros become defaults)
	limits = limits.WithDefaults()

	// Create task set with lock
	var taskSet *global.TaskSet
	err := s.withLock(project, path, func() error {
		// Check if already exists
		filePath := s.getTaskSetFilePath(project, path)
		if _, err := os.Stat(filePath); err == nil {
			return fmt.Errorf("task set already exists: %s", path)
		}

		now := time.Now()
		taskSet = &global.TaskSet{
			Path:        path,
			Title:       title,
			Description: description,
			Parallel:    parallel,
			Limits:      limits,
			CreatedAt:   now,
			UpdatedAt:   now,
			Tasks:       []global.Task{},
		}

		// Apply templates if provided
		if templates != nil {
			taskSet.WorkerResponseTemplate = templates.WorkerResponseTemplate
			taskSet.WorkerReportTemplate = templates.WorkerReportTemplate
			taskSet.QAResponseTemplate = templates.QAResponseTemplate
			taskSet.QAReportTemplate = templates.QAReportTemplate
		}

		return s.saveTaskSet(project, path, taskSet)
	})

	if err != nil {
		return nil, err
	}

	s.logger.Infof("Created task set: project=%s path=%s", project, path)
	return taskSet, nil
}

// GetTaskSet retrieves a task set by path
func (s *Service) GetTaskSet(project, path string) (*global.TaskSet, error) {
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	var taskSet *global.TaskSet
	err := s.withLock(project, path, func() error {
		var err error
		taskSet, err = s.loadTaskSet(project, path)
		return err
	})

	if err != nil {
		return nil, err
	}

	return taskSet, nil
}

// ListTaskSets lists all task sets for a project, optionally filtered by path prefix
func (s *Service) ListTaskSets(project, pathPrefix string) (*TaskSetListResult, error) {
	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	if pathPrefix != "" {
		if err := validatePath(pathPrefix); err != nil {
			return nil, fmt.Errorf("invalid path prefix: %w", err)
		}
	}

	tasksDir := s.projects.GetTasksDir(project)
	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		return &TaskSetListResult{
			TaskSets: []*global.TaskSet{},
			Total:    0,
		}, nil
	}

	// Read all JSON files in tasks directory
	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %w", err)
	}

	taskSets := []*global.TaskSet{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip lock files
		if strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}

		// Convert filename back to path (e.g., "analysis-code.json" -> "analysis/code")
		filename := strings.TrimSuffix(entry.Name(), ".json")
		path := strings.ReplaceAll(filename, global.ListPathSeparator, global.TaskPathSeparator)

		// Filter by path prefix if provided
		if pathPrefix != "" {
			if !strings.HasPrefix(path, pathPrefix) {
				continue
			}
		}

		// Load task set (with basic error handling - skip corrupted files)
		taskSet, err := s.loadTaskSet(project, path)
		if err != nil {
			s.logger.Warnf("Failed to load task set %s: %v", path, err)
			continue
		}

		taskSets = append(taskSets, taskSet)
	}

	// Sort by path
	sort.Slice(taskSets, func(i, j int) bool {
		return taskSets[i].Path < taskSets[j].Path
	})

	return &TaskSetListResult{
		TaskSets: taskSets,
		Total:    len(taskSets),
	}, nil
}

// UpdateTaskSet updates task set metadata
func (s *Service) UpdateTaskSet(project, path string, title, description *string, templates *global.DefaultTemplates, parallel *bool, limits *global.Limits) (*global.TaskSet, error) {
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	var taskSet *global.TaskSet
	err := s.withLock(project, path, func() error {
		var err error
		taskSet, err = s.loadTaskSet(project, path)
		if err != nil {
			return err
		}

		// Update fields
		if title != nil {
			if *title == "" {
				return fmt.Errorf("title cannot be empty")
			}
			taskSet.Title = *title
		}

		if description != nil {
			taskSet.Description = *description
		}

		if templates != nil {
			taskSet.WorkerResponseTemplate = templates.WorkerResponseTemplate
			taskSet.WorkerReportTemplate = templates.WorkerReportTemplate
			taskSet.QAResponseTemplate = templates.QAResponseTemplate
			taskSet.QAReportTemplate = templates.QAReportTemplate
		}

		if parallel != nil {
			taskSet.Parallel = *parallel
		}

		if limits != nil {
			taskSet.Limits = *limits
		}

		taskSet.UpdatedAt = time.Now()
		return s.saveTaskSet(project, path, taskSet)
	})

	if err != nil {
		return nil, err
	}

	s.logger.Infof("Updated task set: project=%s path=%s", project, path)
	return taskSet, nil
}

// DeleteTaskSet deletes a task set and all its tasks
func (s *Service) DeleteTaskSet(project, path string) error {
	if err := validatePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	err := s.withLock(project, path, func() error {
		filePath := s.getTaskSetFilePath(project, path)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("task set not found: %s", path)
		}

		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to delete task set: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Clean up lock file
	lockPath := s.getLockPath(project, path)
	_ = os.Remove(lockPath)

	s.logger.Infof("Deleted task set: project=%s path=%s", project, path)
	return nil
}

// CreateTask creates a new task in a task set
func (s *Service) CreateTask(project, path, title, taskType string, work *global.WorkExecution, qa *global.QAExecution) (*global.Task, error) {
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if title == "" {
		return nil, fmt.Errorf("title cannot be empty")
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	if work == nil {
		return nil, fmt.Errorf("work execution cannot be nil")
	}

	// At least one prompt field is required
	if work.Prompt == "" && work.InstructionsFile == "" && work.InstructionsText == "" {
		return nil, fmt.Errorf("at least one prompt field is required: instructions_file, instructions_text, or prompt")
	}

	var task *global.Task
	err := s.withLock(project, path, func() error {
		taskSet, err := s.loadTaskSet(project, path)
		if err != nil {
			return err
		}

		now := time.Now()
		taskID := getNextTaskID(taskSet.Tasks)

		// Initialize work defaults
		if work.Status == "" {
			work.Status = global.ExecutionStatusWaiting
		}

		// Initialize QA if provided
		qaExec := global.QAExecution{Enabled: false}
		if qa != nil {
			qaExec = *qa
			if qaExec.Enabled && qaExec.Status == "" {
				qaExec.Status = global.ExecutionStatusWaiting
			}
		}

		task = &global.Task{
			ID:        taskID,
			UUID:      uuid.New().String(),
			Title:     title,
			Type:      taskType,
			CreatedAt: now,
			UpdatedAt: now,
			Work:      *work,
			QA:        qaExec,
		}

		taskSet.Tasks = append(taskSet.Tasks, *task)
		taskSet.UpdatedAt = now

		if err := s.saveTaskSet(project, path, taskSet); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Infof("Created task: project=%s path=%s id=%d uuid=%s", project, path, task.ID, task.UUID)
	return task, nil
}

// GetTask retrieves a task by UUID (searches all task sets)
func (s *Service) GetTask(project, taskUUID string) (*global.Task, string, error) {
	if !s.projects.ProjectExists(project) {
		return nil, "", fmt.Errorf("project not found: %s", project)
	}

	// List all task sets
	result, err := s.ListTaskSets(project, "")
	if err != nil {
		return nil, "", err
	}

	// Search each task set for the UUID
	for _, taskSet := range result.TaskSets {
		_, task := findTaskByUUID(taskSet.Tasks, taskUUID)
		if task != nil {
			return task, taskSet.Path, nil
		}
	}

	return nil, "", fmt.Errorf("task not found: %s", taskUUID)
}

// GetTaskByID retrieves a task by ID within a specific task set
func (s *Service) GetTaskByID(project, path string, taskID int) (*global.Task, error) {
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	var task *global.Task
	err := s.withLock(project, path, func() error {
		taskSet, err := s.loadTaskSet(project, path)
		if err != nil {
			return err
		}

		_, foundTask := findTaskByID(taskSet.Tasks, taskID)
		if foundTask == nil {
			return fmt.Errorf("task not found: id=%d", taskID)
		}

		task = foundTask
		return nil
	})

	if err != nil {
		return nil, err
	}

	return task, nil
}

// ListTasks lists tasks with optional filters
func (s *Service) ListTasks(project, path, statusFilter, typeFilter string, limit, offset int) (*TaskListResult, error) {
	if err := validatePath(path); err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	var tasks []*global.Task
	err := s.withLock(project, path, func() error {
		taskSet, err := s.loadTaskSet(project, path)
		if err != nil {
			return err
		}

		// Filter and collect tasks
		for i := range taskSet.Tasks {
			task := &taskSet.Tasks[i]

			// Apply filters
			if statusFilter != "" && task.Work.Status != statusFilter {
				continue
			}
			if typeFilter != "" && task.Type != typeFilter {
				continue
			}

			tasks = append(tasks, task)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Apply pagination
	total := len(tasks)
	if offset >= total {
		tasks = []*global.Task{}
	} else {
		end := offset + limit
		if limit <= 0 || end > total {
			end = total
		}
		tasks = tasks[offset:end]
	}

	return &TaskListResult{
		Tasks: tasks,
		Total: total,
		Path:  path,
	}, nil
}

// UpdateTask updates a task by UUID
func (s *Service) UpdateTask(project, taskUUID string, updates map[string]interface{}) (*global.Task, error) {
	if !s.projects.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	// Find the task set containing this task
	result, err := s.ListTaskSets(project, "")
	if err != nil {
		return nil, err
	}

	var targetPath string
	for _, taskSet := range result.TaskSets {
		_, task := findTaskByUUID(taskSet.Tasks, taskUUID)
		if task != nil {
			targetPath = taskSet.Path
			break
		}
	}

	if targetPath == "" {
		return nil, fmt.Errorf("task not found: %s", taskUUID)
	}

	// Update the task
	var updatedTask *global.Task
	err = s.withLock(project, targetPath, func() error {
		taskSet, err := s.loadTaskSet(project, targetPath)
		if err != nil {
			return err
		}

		idx, task := findTaskByUUID(taskSet.Tasks, taskUUID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskUUID)
		}

		// Apply updates
		if title, ok := updates["title"].(string); ok {
			if title == "" {
				return fmt.Errorf("title cannot be empty")
			}
			task.Title = title
		}

		if taskType, ok := updates["type"].(string); ok {
			task.Type = taskType
		}

		// Update work fields if provided
		if workUpdates, ok := updates["work"].(map[string]interface{}); ok {
			if status, ok := workUpdates["status"].(string); ok {
				task.Work.Status = status
			}
			// Note: result is stored in results/<uuid>.json, not in tasks.json
			if errMsg, ok := workUpdates["error"].(string); ok {
				task.Work.Error = errMsg
			}
			if invocations, ok := workUpdates["invocations"].(int); ok {
				task.Work.Invocations = invocations
			}
			if lastAttemptAt, ok := workUpdates["last_attempt_at"].(*time.Time); ok {
				task.Work.LastAttemptAt = lastAttemptAt
			}
			// Instruction and prompt fields
			if instructionsFile, ok := workUpdates["instructions_file"].(string); ok {
				task.Work.InstructionsFile = instructionsFile
			}
			if instructionsFileSource, ok := workUpdates["instructions_file_source"].(string); ok {
				task.Work.InstructionsFileSource = instructionsFileSource
			}
			if instructionsText, ok := workUpdates["instructions_text"].(string); ok {
				task.Work.InstructionsText = instructionsText
			}
			if prompt, ok := workUpdates["prompt"].(string); ok {
				task.Work.Prompt = prompt
			}
			if llmModelID, ok := workUpdates["llm_model_id"].(string); ok {
				task.Work.LLMModelID = llmModelID
			}
		}

		// Update QA fields if provided
		if qaUpdates, ok := updates["qa"].(map[string]interface{}); ok {
			if status, ok := qaUpdates["status"].(string); ok {
				task.QA.Status = status
			}
			// Note: result is stored in results/<uuid>.json, not in tasks.json
			if verdict, ok := qaUpdates["verdict"].(string); ok {
				task.QA.Verdict = verdict
			}
			if errorMsg, ok := qaUpdates["error"].(string); ok {
				task.QA.Error = errorMsg
			}
			if invocations, ok := qaUpdates["invocations"].(int); ok {
				task.QA.Invocations = invocations
			}
			// Instruction and prompt fields
			if instructionsFile, ok := qaUpdates["instructions_file"].(string); ok {
				task.QA.InstructionsFile = instructionsFile
			}
			if instructionsFileSource, ok := qaUpdates["instructions_file_source"].(string); ok {
				task.QA.InstructionsFileSource = instructionsFileSource
			}
			if instructionsText, ok := qaUpdates["instructions_text"].(string); ok {
				task.QA.InstructionsText = instructionsText
			}
			if prompt, ok := qaUpdates["prompt"].(string); ok {
				task.QA.Prompt = prompt
			}
			if llmModelID, ok := qaUpdates["llm_model_id"].(string); ok {
				task.QA.LLMModelID = llmModelID
			}
		}

		task.UpdatedAt = time.Now()
		taskSet.Tasks[idx] = *task
		taskSet.UpdatedAt = time.Now()

		if err := s.saveTaskSet(project, targetPath, taskSet); err != nil {
			return err
		}

		updatedTask = task
		return nil
	})

	if err != nil {
		return nil, err
	}

	s.logger.Infof("Updated task: project=%s uuid=%s", project, taskUUID)
	return updatedTask, nil
}

// DeleteTask deletes a task by UUID
func (s *Service) DeleteTask(project, taskUUID string) error {
	if !s.projects.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	// Find the task set containing this task
	result, err := s.ListTaskSets(project, "")
	if err != nil {
		return err
	}

	var targetPath string
	for _, taskSet := range result.TaskSets {
		_, task := findTaskByUUID(taskSet.Tasks, taskUUID)
		if task != nil {
			targetPath = taskSet.Path
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("task not found: %s", taskUUID)
	}

	// Delete the task
	err = s.withLock(project, targetPath, func() error {
		taskSet, err := s.loadTaskSet(project, targetPath)
		if err != nil {
			return err
		}

		idx, task := findTaskByUUID(taskSet.Tasks, taskUUID)
		if task == nil {
			return fmt.Errorf("task not found: %s", taskUUID)
		}

		// Remove task from slice
		taskSet.Tasks = append(taskSet.Tasks[:idx], taskSet.Tasks[idx+1:]...)
		taskSet.UpdatedAt = time.Now()

		return s.saveTaskSet(project, targetPath, taskSet)
	})

	if err != nil {
		return err
	}

	s.logger.Infof("Deleted task: project=%s uuid=%s", project, taskUUID)
	return nil
}

// Helper functions

// getNextTaskID returns the next sequential task ID for a task set
func getNextTaskID(tasks []global.Task) int {
	maxID := 0
	for _, task := range tasks {
		if task.ID > maxID {
			maxID = task.ID
		}
	}
	return maxID + 1
}

// findTaskByID finds a task by ID in a slice
func findTaskByID(tasks []global.Task, taskID int) (int, *global.Task) {
	for i := range tasks {
		if tasks[i].ID == taskID {
			return i, &tasks[i]
		}
	}
	return -1, nil
}

// findTaskByUUID finds a task by UUID in a slice
func findTaskByUUID(tasks []global.Task, taskUUID string) (int, *global.Task) {
	for i := range tasks {
		if tasks[i].UUID == taskUUID {
			return i, &tasks[i]
		}
	}
	return -1, nil
}

// Exposed methods for runner and other packages

// GetResultsDir returns the results directory for a project
func (s *Service) GetResultsDir(project string) string {
	return s.projects.GetResultsDir(project)
}

// AppendLog appends a message to the project log
func (s *Service) AppendLog(project, message string) error {
	return s.projects.AppendLog(project, "", message)
}

// GetProjectFile gets a file from a project's files directory
func (s *Service) GetProjectFile(project, path string) (string, error) {
	item, err := s.projects.GetFile(project, path, 0, 0)
	if err != nil {
		return "", err
	}
	return item.Content, nil
}

// PutProjectFile writes a file to a project's files directory
func (s *Service) PutProjectFile(project, path, content, summary string) error {
	_, err := s.projects.PutFile(project, path, content, summary)
	return err
}

// AppendHistory is deprecated - history is now stored in results/<uuid>.json files
// The TaskSetRunner accumulates history in memory and writes it when the task completes
func (s *Service) AppendHistory(project, taskUUID string, message global.Message) error {
	// No-op: history is now managed by TaskSetRunner and stored in results files
	return nil
}

// ProjectExists checks if a project exists
func (s *Service) ProjectExists(project string) bool {
	return s.projects.ProjectExists(project)
}

// GetMutex returns the mutex for a project (used by runner)
func (s *Service) GetMutex(project string) *sync.Mutex {
	return s.projects.GetMutex(project)
}

// ResetTaskSet resets tasks in a task set based on the mode parameter.
// mode must be "all" (reset all tasks) or "failed" (reset only failed tasks).
// Returns the updated task set and the count of tasks that were reset.
func (s *Service) ResetTaskSet(project, path, mode string, deleteResults bool) (*global.TaskSet, int, error) {
	if err := validatePath(path); err != nil {
		return nil, 0, fmt.Errorf("invalid path: %w", err)
	}

	if !s.projects.ProjectExists(project) {
		return nil, 0, fmt.Errorf("project not found: %s", project)
	}

	// Validate mode parameter
	if mode != "all" && mode != "failed" {
		return nil, 0, fmt.Errorf("mode is required: specify 'all' to reset all tasks or 'failed' to reset only failed tasks")
	}

	var taskSet *global.TaskSet
	var resetCount int
	err := s.withLock(project, path, func() error {
		var err error
		taskSet, err = s.loadTaskSet(project, path)
		if err != nil {
			return err
		}

		now := time.Now()
		resultsDir := s.GetResultsDir(project)

		// Reset tasks based on mode
		for i := range taskSet.Tasks {
			task := &taskSet.Tasks[i]

			// Check if this task should be reset
			shouldReset := false
			if mode == "all" {
				shouldReset = true
			} else if mode == "failed" {
				// Reset only tasks that are in failed or error status
				shouldReset = task.Work.Status == global.ExecutionStatusFailed ||
					task.Work.Status == global.ExecutionStatusError ||
					(task.QA.Enabled && (task.QA.Status == global.ExecutionStatusFailed ||
						task.QA.Status == global.ExecutionStatusError))
			}

			if !shouldReset {
				continue
			}

			resetCount++

			// Reset work phase
			task.Work.Status = global.ExecutionStatusWaiting
			task.Work.Invocations = 0
			task.Work.Error = ""
			task.Work.LastAttemptAt = nil

			// Reset QA phase if enabled
			if task.QA.Enabled {
				task.QA.Status = global.ExecutionStatusWaiting
				task.QA.Invocations = 0
				task.QA.Error = ""
				task.QA.Verdict = ""
			}

			task.UpdatedAt = now

			// Delete results file if requested
			if deleteResults && resultsDir != "" {
				resultFile := filepath.Join(resultsDir, task.UUID+".json")
				if err := os.Remove(resultFile); err != nil && !os.IsNotExist(err) {
					s.logger.Warnf("Failed to delete result file %s: %v", resultFile, err)
				}
				// Also delete any error files
				errorFile := filepath.Join(resultsDir, task.UUID+"-error.json")
				if err := os.Remove(errorFile); err != nil && !os.IsNotExist(err) {
					s.logger.Warnf("Failed to delete error file %s: %v", errorFile, err)
				}
			}
		}

		taskSet.UpdatedAt = now

		// Save the updated task set
		return s.saveTaskSet(project, path, taskSet)
	})

	if err != nil {
		return nil, 0, err
	}

	return taskSet, resetCount, nil
}
