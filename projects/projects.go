/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"bufio"
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
	"github.com/google/uuid"
)

// Service provides project management operations
type Service struct {
	config       *config.Config
	logger       *logging.Logger
	projectMutex sync.Map // map[string]*sync.Mutex for per-project locking
}

// ProjectInfo is returned by List operations
type ProjectInfo struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ProjectListResult is the response for project_list
//
//goland:noinspection GoNameStartsWithPackageName
type ProjectListResult struct {
	Projects []*ProjectInfo `json:"projects"`
	Total    int            `json:"total"`
}

// LogResult is the response for log_get
type LogResult struct {
	Project string   `json:"project"`
	Task    string   `json:"task,omitempty"`
	Events  []string `json:"events"`
}

// projectNameRegex validates project/subproject names
var projectNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// NewService creates a new projects service
func NewService(cfg *config.Config, logger *logging.Logger) *Service {
	return &Service{
		config:       cfg,
		logger:       logger,
		projectMutex: sync.Map{},
	}
}

// getProjectMutex gets or creates a mutex for a specific project
func (s *Service) getProjectMutex(project string) *sync.Mutex {
	value, _ := s.projectMutex.LoadOrStore(project, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// getProjectDir returns the directory path for a project
func (s *Service) getProjectDir(project string) string {
	return filepath.Join(s.config.ProjectsDir(), project)
}

// getProjectFilePath returns the path to the project.json file
func (s *Service) getProjectFilePath(project string) string {
	return filepath.Join(s.getProjectDir(project), global.ProjectFileName)
}

// getProjectLogPath returns the path to the project log file
func (s *Service) getProjectLogPath(project string) string {
	return filepath.Join(s.getProjectDir(project), global.ProjectLogName)
}

// getResultsDir returns the path to the results directory
func (s *Service) getResultsDir(project string) string {
	return filepath.Join(s.getProjectDir(project), "results")
}

// GetFilesDir returns the path to the project files directory.
// Returns empty string if project doesn't exist.
func (s *Service) GetFilesDir(project string) string {
	if !s.ProjectExists(project) {
		return ""
	}
	return filepath.Join(s.getProjectDir(project), "files")
}

// validateProjectName validates a project or subproject name
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if len(name) > global.DefaultProjectNameMaxLen {
		return fmt.Errorf("project name exceeds maximum length of %d characters", global.DefaultProjectNameMaxLen)
	}
	if !projectNameRegex.MatchString(name) {
		return fmt.Errorf("invalid project name: must be alphanumeric with hyphens or underscores, cannot start with . or _")
	}
	return nil
}

// validateProjectStatus validates a project status value
func validateProjectStatus(status string) error {
	validStatuses := map[string]bool{
		global.ProjectStatusPending:    true,
		global.ProjectStatusInProgress: true,
		global.ProjectStatusDone:       true,
		global.ProjectStatusCancelled:  true,
	}
	if !validStatuses[status] {
		return fmt.Errorf("invalid project status: %s (valid: pending, in_progress, done, cancelled)", status)
	}
	return nil
}

// loadProject loads a project file
func (s *Service) loadProject(project string) (*global.Project, error) {
	projectPath := s.getProjectFilePath(project)

	data, err := os.ReadFile(projectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("project not found: %s", project)
		}
		return nil, fmt.Errorf("failed to read project file: %w", err)
	}

	var proj global.Project
	if err := json.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("failed to parse project file: %w", err)
	}

	return &proj, nil
}

// saveProject saves a project file atomically
func (s *Service) saveProject(project string, proj *global.Project) error {
	projectDir := s.getProjectDir(project)
	projectPath := s.getProjectFilePath(project)

	// Ensure directory exists
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Create results directory
	resultsDir := s.getResultsDir(project)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	// Create tasks directory
	tasksDir := filepath.Join(projectDir, global.TasksDir)
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		return fmt.Errorf("failed to create tasks directory: %w", err)
	}

	// Create files directory
	filesDir := filepath.Join(projectDir, "files")
	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return fmt.Errorf("failed to create files directory: %w", err)
	}

	// Marshal project
	data, err := json.MarshalIndent(proj, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project: %w", err)
	}

	// Atomic write using temp file
	tempPath := projectPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp project file: %w", err)
	}

	if err := os.Rename(tempPath, projectPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to rename project file: %w", err)
	}

	return nil
}

// Create creates a new project
func (s *Service) Create(project, title, description, projectContext, status, disclaimerTemplate string) (*global.Project, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("project title cannot be empty")
	}
	if status == "" {
		status = global.ProjectStatusPending
	}
	if err := validateProjectStatus(status); err != nil {
		return nil, err
	}

	// Validate disclaimer_template is provided and valid
	if disclaimerTemplate == "" {
		return nil, fmt.Errorf("disclaimer_template is required: provide a playbook path (e.g., 'playbook-name/templates/disclaimer.md') or 'none'")
	}
	if disclaimerTemplate != "none" {
		if err := s.validateDisclaimerPath(disclaimerTemplate); err != nil {
			return nil, err
		}
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if project already exists
	projectPath := s.getProjectFilePath(project)
	if _, err := os.Stat(projectPath); err == nil {
		return nil, fmt.Errorf("project already exists: %s", project)
	}

	now := time.Now()
	proj := &global.Project{
		UUID:               uuid.New().String(),
		Name:               project,
		Title:              title,
		Description:        description,
		Context:            projectContext,
		Status:             status,
		DisclaimerTemplate: disclaimerTemplate,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.saveProject(project, proj); err != nil {
		return nil, err
	}

	// Create initial log entry
	if err := s.appendLogEntry(project, "Project created"); err != nil {
		s.logger.Warnf("Failed to create initial log entry: %v", err)
	}

	s.logger.Debugf("Created project: %s", project)
	return proj, nil
}

// validateDisclaimerPath validates that a disclaimer template path exists.
// Path format: "playbook-name/path/to/file.md"
func (s *Service) validateDisclaimerPath(disclaimerPath string) error {
	parts := strings.SplitN(disclaimerPath, "/", 2)
	if len(parts) < 2 {
		return fmt.Errorf("invalid disclaimer_template format: must be 'playbook-name/path/to/file.md', got: %s", disclaimerPath)
	}

	playbookName := parts[0]
	filePath := parts[1]

	// Build full path
	fullPath := filepath.Join(s.config.PlaybooksDir(), playbookName, filePath)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("disclaimer template not found: %s", disclaimerPath)
	}

	return nil
}

// Get retrieves a project
func (s *Service) Get(project string) (*global.Project, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	proj, err := s.loadProject(project)
	if err != nil {
		return nil, err
	}

	return proj, nil
}

// Update updates project metadata
func (s *Service) Update(project string, title, description, projectContext, status, disclaimerTemplate *string) (*global.Project, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	proj, err := s.loadProject(project)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if title != nil {
		proj.Title = *title
	}
	if description != nil {
		proj.Description = *description
	}
	if projectContext != nil {
		proj.Context = *projectContext
	}
	if status != nil {
		if err := validateProjectStatus(*status); err != nil {
			return nil, err
		}
		proj.Status = *status
	}
	if disclaimerTemplate != nil {
		proj.DisclaimerTemplate = *disclaimerTemplate
	}

	proj.UpdatedAt = time.Now()

	if err := s.saveProject(project, proj); err != nil {
		return nil, err
	}

	s.logger.Debugf("Updated project: %s", project)
	return proj, nil
}

// List lists all projects with optional status filter
func (s *Service) List(status string, limit, offset int) (*ProjectListResult, error) {
	if limit <= 0 {
		limit = global.DefaultLimit
	}

	// Find all project directories (direct children of projects dir)
	entries, err := os.ReadDir(s.config.ProjectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectListResult{
				Projects: []*ProjectInfo{},
				Total:    0,
			}, nil
		}
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	var allProjects []*ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectName := entry.Name()

		// Verify it has a project.json file
		projectPath := s.getProjectFilePath(projectName)
		if _, err := os.Stat(projectPath); err != nil {
			continue
		}

		// Load project to get metadata
		proj, err := s.loadProject(projectName)
		if err != nil {
			s.logger.Warnf("Failed to load project %s: %v", projectName, err)
			continue
		}

		// Apply status filter
		if status != "" && proj.Status != status {
			continue
		}

		allProjects = append(allProjects, &ProjectInfo{
			Name:      projectName,
			Title:     proj.Title,
			Status:    proj.Status,
			CreatedAt: proj.CreatedAt.Format(time.RFC3339),
			UpdatedAt: proj.UpdatedAt.Format(time.RFC3339),
		})
	}

	// Sort by updated_at descending (most recent first)
	sort.Slice(allProjects, func(i, j int) bool {
		return allProjects[i].UpdatedAt > allProjects[j].UpdatedAt
	})

	// Apply pagination
	total := len(allProjects)
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	var projects []*ProjectInfo
	if start < total {
		projects = allProjects[start:end]
	} else {
		projects = []*ProjectInfo{}
	}

	s.logger.Debugf("Listed %d projects (total: %d)", len(projects), total)

	return &ProjectListResult{
		Projects: projects,
		Total:    total,
	}, nil
}

// Rename renames a project
func (s *Service) Rename(project, newName string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}
	if err := validateProjectName(newName); err != nil {
		return fmt.Errorf("invalid new name: %w", err)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	oldDir := s.getProjectDir(project)

	// Check source exists
	if _, err := os.Stat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("project not found: %s", project)
	}

	// Renaming a project
	newDir := filepath.Join(s.config.ProjectsDir(), newName)

	// Check destination doesn't exist
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("project already exists: %s", newName)
	}

	// Rename directory
	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("failed to rename project: %w", err)
	}

	// Update project.json to reflect new name
	proj, loadErr := s.loadProject(newName) // Load from new location

	if loadErr == nil {
		proj.Name = newName
		proj.UpdatedAt = time.Now()
		if err := s.saveProject(newName, proj); err != nil {
			s.logger.Warnf("Failed to update project.json after rename: %v", err)
		}
	}

	s.logger.Debugf("Renamed project: %s -> %s", project, newName)
	return nil
}

// Delete deletes a project and all its logs and results
func (s *Service) Delete(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	projectDir := s.getProjectDir(project)

	// Check if project exists
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return fmt.Errorf("project not found: %s", project)
	}

	// Delete the directory recursively
	if err := os.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("failed to delete project directory: %w", err)
	}

	s.logger.Debugf("Deleted project: %s", project)
	return nil
}

// appendLogEntry appends a log entry to the project log file
func (s *Service) appendLogEntry(project, message string) error {
	logPath := s.getProjectLogPath(project)

	// Ensure directory exists
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Open file in append mode
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	// Write log entry
	timestamp := time.Now().Format(time.RFC3339)
	entry := fmt.Sprintf("%s %s\n", timestamp, message)
	if _, err := f.WriteString(entry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

// AppendLog appends a log entry to the project or task log
func (s *Service) AppendLog(project, taskID, message string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}
	if message == "" {
		return fmt.Errorf("log message cannot be empty")
	}

	// Verify project exists
	projectPath := s.getProjectFilePath(project)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("project not found: %s", project)
	}

	// If task ID is provided, log to task-specific log
	if taskID != "" {
		// Task logs are stored with their results
		taskLogPath := filepath.Join(s.getResultsDir(project), fmt.Sprintf("task-%s.log", taskID))

		// Ensure directory exists
		dir := filepath.Dir(taskLogPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create results directory: %w", err)
		}

		f, err := os.OpenFile(taskLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open task log file: %w", err)
		}
		defer func(f *os.File) {
			_ = f.Close()
		}(f)

		timestamp := time.Now().Format(time.RFC3339)
		entry := fmt.Sprintf("%s %s\n", timestamp, message)
		if _, err := f.WriteString(entry); err != nil {
			return fmt.Errorf("failed to write task log entry: %w", err)
		}

		s.logger.Debugf("Appended task log: %s task %s", project, taskID)
		return nil
	}

	// Log to project log
	if err := s.appendLogEntry(project, message); err != nil {
		return err
	}

	s.logger.Debugf("Appended log to project: %s", project)
	return nil
}

// GetLog retrieves the project or task log
func (s *Service) GetLog(project, taskID string, limit, offset int) (*LogResult, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	// Verify project exists
	projectPath := s.getProjectFilePath(project)
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	var logPath string
	if taskID != "" {
		logPath = filepath.Join(s.getResultsDir(project), fmt.Sprintf("task-%s.log", taskID))
	} else {
		logPath = s.getProjectLogPath(project)
	}

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return &LogResult{
			Project: project,
			Task:    taskID,
			Events:  []string{},
		}, nil
	}

	// Open and read log file
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	var allEvents []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allEvents = append(allEvents, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Apply offset and limit
	total := len(allEvents)
	start := offset
	if start > total {
		start = total
	}

	var end int
	if limit <= 0 {
		end = total
	} else {
		end = start + limit
		if end > total {
			end = total
		}
	}

	var events []string
	if start < total {
		events = allEvents[start:end]
	} else {
		events = []string{}
	}

	s.logger.Debugf("Retrieved %d log events for project: %s (task: %s)", len(events), project, taskID)

	return &LogResult{
		Project: project,
		Task:    taskID,
		Events:  events,
	}, nil
}

// GetProjectForTasks returns the project for task operations (used by tasks package)
// NOTE: Caller must hold the project mutex
func (s *Service) GetProjectForTasks(project string) (*global.Project, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	return s.loadProject(project)
}

// SaveProjectForTasks saves the project after task operations (used by tasks package)
// NOTE: Caller must hold the project mutex
func (s *Service) SaveProjectForTasks(project string, proj *global.Project) error {
	return s.saveProject(project, proj)
}

// ProjectExists checks if a project exists
func (s *Service) ProjectExists(project string) bool {
	if err := validateProjectName(project); err != nil {
		return false
	}
	projectPath := s.getProjectFilePath(project)
	_, err := os.Stat(projectPath)
	return err == nil
}

// GetMutex returns the mutex for a project (used by tasks package)
func (s *Service) GetMutex(project string) *sync.Mutex {
	return s.getProjectMutex(project)
}

// GetResultsDir returns the results directory path (used by tasks package)
func (s *Service) GetResultsDir(project string) string {
	return s.getResultsDir(project)
}

// GetTasksDir returns the tasks directory path (used by tasks package)
func (s *Service) GetTasksDir(project string) string {
	return filepath.Join(s.getProjectDir(project), global.TasksDir)
}

// AddToManifest adds a taskset to the report manifest.
// If the taskset is already in the manifest, this is a no-op.
// Returns the sequence number assigned (or existing sequence if already present).
func (s *Service) AddToManifest(project, tasksetPath string) (int, error) {
	if err := validateProjectName(project); err != nil {
		return 0, err
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	proj, err := s.loadProject(project)
	if err != nil {
		return 0, err
	}

	// Check if already in manifest
	for _, entry := range proj.ReportManifest {
		if entry.Path == tasksetPath {
			return entry.Sequence, nil // Already exists
		}
	}

	// Increment sequence and add entry
	proj.ReportSequence++
	entry := global.ReportManifestEntry{
		Path:     tasksetPath,
		Sequence: proj.ReportSequence,
	}
	proj.ReportManifest = append(proj.ReportManifest, entry)
	proj.UpdatedAt = time.Now()

	if err := s.saveProject(project, proj); err != nil {
		return 0, err
	}

	s.logger.Debugf("Added taskset '%s' to manifest with sequence %d", tasksetPath, proj.ReportSequence)
	return proj.ReportSequence, nil
}

// ClearManifest clears the report manifest (used when starting fresh report generation).
func (s *Service) ClearManifest(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	proj, err := s.loadProject(project)
	if err != nil {
		return err
	}

	proj.ReportManifest = nil
	proj.ReportSequence = 0
	proj.UpdatedAt = time.Now()

	if err := s.saveProject(project, proj); err != nil {
		return err
	}

	s.logger.Debugf("Cleared report manifest for project %s", project)
	return nil
}

// GetManifest returns the report manifest sorted by sequence.
func (s *Service) GetManifest(project string) ([]global.ReportManifestEntry, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	proj, err := s.Get(project)
	if err != nil {
		return nil, err
	}

	// Return copy sorted by sequence
	manifest := make([]global.ReportManifestEntry, len(proj.ReportManifest))
	copy(manifest, proj.ReportManifest)

	// Sort by sequence
	for i := 0; i < len(manifest)-1; i++ {
		for j := i + 1; j < len(manifest); j++ {
			if manifest[i].Sequence > manifest[j].Sequence {
				manifest[i], manifest[j] = manifest[j], manifest[i]
			}
		}
	}

	return manifest, nil
}
