/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PivotLLM/Maestro/global"
)

// ReportItem represents a report file within a project's reports directory.
type ReportItem struct {
	Project    string `json:"project"`
	Name       string `json:"name"`
	SizeBytes  int64  `json:"size_bytes"`
	ModifiedAt string `json:"modified_at"`
	Content    string `json:"content,omitempty"`
	// Byte range fields (only set when offset/max_bytes used)
	Offset     int64 `json:"offset,omitempty"`
	TotalBytes int64 `json:"total_bytes,omitempty"`
}

// getReportsDir returns the path to the reports directory for a project.
func (s *Service) getReportsDir(project string) string {
	return filepath.Join(s.getProjectDir(project), global.ReportsDir)
}

// validateReportName validates a report name (no path traversal, flat directory).
func validateReportName(name string) error {
	if name == "" {
		return fmt.Errorf("report name cannot be empty")
	}
	// Reports must be flat - no subdirectories
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("report name cannot contain path separators (reports directory is flat)")
	}
	// Check for path traversal attempts
	if strings.Contains(name, "..") {
		return fmt.Errorf("report name cannot contain '..'")
	}
	// Must end with .md
	if !strings.HasSuffix(name, ".md") {
		return fmt.Errorf("report name must end with .md")
	}
	return nil
}

// ListReports lists all reports in a project.
func (s *Service) ListReports(project string) ([]ReportItem, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	if !s.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	reportsDir := s.getReportsDir(project)

	// Check if reports directory exists
	if !global.DirExists(reportsDir) {
		return []ReportItem{}, nil
	}

	var items []ReportItem

	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read reports directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories (shouldn't exist, but be safe)
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".md") {
			continue // Only markdown reports
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		items = append(items, ReportItem{
			Project:    project,
			Name:       name,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	s.logger.Debugf("Listed %d reports in project '%s'", len(items), project)
	return items, nil
}

// ReadReport retrieves a report from a project with optional byte range.
func (s *Service) ReadReport(project, name string, offset, maxBytes int64) (*ReportItem, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	if err := validateReportName(name); err != nil {
		return nil, err
	}

	if !s.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	reportsDir := s.getReportsDir(project)
	absPath := filepath.Join(reportsDir, name)

	// Verify path is within reports directory (defense in depth)
	if !strings.HasPrefix(absPath, reportsDir) {
		return nil, fmt.Errorf("invalid report path")
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("report not found: %s", name)
		}
		return nil, fmt.Errorf("failed to stat report: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a report: %s", name)
	}

	// Read content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read report: %w", err)
	}

	totalBytes := info.Size()

	// Apply byte range if specified
	var resultContent string
	var resultOffset int64

	if maxBytes > 0 {
		if offset < 0 {
			offset = 0
		}
		if offset >= int64(len(content)) {
			resultContent = ""
			resultOffset = offset
		} else {
			end := offset + maxBytes
			if end > int64(len(content)) {
				end = int64(len(content))
			}
			resultContent = string(content[offset:end])
			resultOffset = offset
		}
	} else {
		resultContent = string(content)
		resultOffset = 0
	}

	item := &ReportItem{
		Project:    project,
		Name:       name,
		SizeBytes:  int64(len(resultContent)),
		ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		Content:    resultContent,
		Offset:     resultOffset,
		TotalBytes: totalBytes,
	}

	s.logger.Debugf("Read report from project '%s': %s (offset=%d, bytes=%d, total=%d)", project, name, resultOffset, len(resultContent), totalBytes)
	return item, nil
}

// StartReport initializes a report session with a prefix.
// Stores the title, intro, and date in project config - actual file writing happens on first append.
// Returns the generated prefix.
func (s *Service) StartReport(project, title, intro string) (string, error) {
	if err := validateProjectName(project); err != nil {
		return "", err
	}

	if !s.ProjectExists(project) {
		return "", fmt.Errorf("project not found: %s", project)
	}

	// Generate prefix: YYYYMMDD-HHMM-<sanitized-title>-
	now := time.Now()
	sanitizedTitle := sanitizeTitleForPrefix(title)
	prefix := fmt.Sprintf("%s-%s-", now.Format("20060102-1504"), sanitizedTitle)

	// Update project with report prefix, title, intro, and date
	proj, err := s.Get(project)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}

	proj.ReportPrefix = prefix
	proj.ReportStartedAt = &now
	proj.ReportTitle = title
	proj.ReportIntro = intro
	proj.ReportDate = now.Format("2006-01-02") // Capture date at session start
	proj.UpdatedAt = now

	if err := s.saveProject(project, proj); err != nil {
		return "", fmt.Errorf("failed to save project: %w", err)
	}

	// Create reports directory if it doesn't exist
	reportsDir := s.getReportsDir(project)
	if err := global.EnsureDir(reportsDir); err != nil {
		return "", fmt.Errorf("failed to create reports directory: %w", err)
	}

	s.logger.Infof("Project %s: Starting report generation", project)
	return prefix, nil
}

// AppendReport appends content to a report.
// If reportName is empty, appends to main report (<prefix>Report.md).
// If no report session is active, auto-initializes with project name.
// If the file doesn't exist, adds the L1 header (title) and optional intro first.
func (s *Service) AppendReport(project, content, reportName string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	if content == "" {
		return fmt.Errorf("content cannot be empty")
	}

	// Get project to check/set report prefix
	proj, err := s.Get(project)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Auto-initialize report session if not started
	if proj.ReportPrefix == "" {
		prefix, err := s.StartReport(project, proj.Title, "")
		if err != nil {
			return fmt.Errorf("failed to auto-initialize report session: %w", err)
		}
		proj.ReportPrefix = prefix
		// Re-fetch project to get updated title/intro
		proj, err = s.Get(project)
		if err != nil {
			return fmt.Errorf("failed to get project after init: %w", err)
		}
	}

	// Determine report filename
	var filename string
	if reportName == "" {
		filename = proj.ReportPrefix + "Report.md"
	} else {
		// Sanitize report name
		sanitized := sanitizeTitleForPrefix(reportName)
		filename = proj.ReportPrefix + sanitized + ".md"
	}

	// Validate the resulting filename
	if err := validateReportName(filename); err != nil {
		return err
	}

	reportsDir := s.getReportsDir(project)
	if err := global.EnsureDir(reportsDir); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	absPath := filepath.Join(reportsDir, filename)

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Read existing content if file exists
	var existingContent string
	fileExists := false
	if data, err := os.ReadFile(absPath); err == nil {
		existingContent = string(data)
		fileExists = true
	}

	// If file doesn't exist, add the L1 header with date, optional intro, and disclaimer
	if !fileExists {
		// Use ReportTitle if set, otherwise fall back to project Title
		title := proj.ReportTitle
		if title == "" {
			title = proj.Title
		}

		// Build header: title, issued date, then optional intro
		header := fmt.Sprintf("# %s\n\n", title)

		// Add issued date (use captured date or current date if not set)
		reportDate := proj.ReportDate
		if reportDate == "" {
			reportDate = time.Now().Format("2006-01-02")
		}
		header += fmt.Sprintf("**Issued:** %s\n\n", reportDate)

		// Add intro if present
		if proj.ReportIntro != "" {
			header += proj.ReportIntro + "\n\n"
		}

		// Add disclaimer if configured
		disclaimer := s.loadDisclaimer(proj.DisclaimerTemplate)
		if disclaimer != "" {
			// Strip trailing newlines from disclaimer, then add one
			disclaimer = strings.TrimRight(disclaimer, "\n\r")
			header += disclaimer + "\n\n"
		}

		existingContent = header
	}

	// Append content
	newContent := existingContent + content

	// Write atomically
	if err := global.AtomicWrite(absPath, []byte(newContent)); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	s.logger.Infof("Project %s: Wrote report %s", project, filename)
	return nil
}

// EndReport ends the report session and clears the prefix.
func (s *Service) EndReport(project string) error {
	if err := validateProjectName(project); err != nil {
		return err
	}

	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	proj, err := s.Get(project)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	if proj.ReportPrefix == "" {
		return fmt.Errorf("no active report session")
	}

	proj.ReportPrefix = ""
	proj.ReportStartedAt = nil
	proj.ReportTitle = ""
	proj.ReportIntro = ""
	proj.ReportDate = ""
	proj.UpdatedAt = time.Now()

	if err := s.saveProject(project, proj); err != nil {
		return fmt.Errorf("failed to save project: %w", err)
	}

	s.logger.Infof("Project %s: Report session ended", project)
	return nil
}

// GetReportPrefix returns the current report prefix for a project.
func (s *Service) GetReportPrefix(project string) (string, error) {
	if err := validateProjectName(project); err != nil {
		return "", err
	}

	if !s.ProjectExists(project) {
		return "", fmt.Errorf("project not found: %s", project)
	}

	proj, err := s.Get(project)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}

	return proj.ReportPrefix, nil
}

// sanitizeTitleForPrefix converts a title to a safe prefix component.
func sanitizeTitleForPrefix(title string) string {
	if title == "" {
		return "Report"
	}

	// Replace spaces and special characters with hyphens
	result := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1 // Remove other characters
	}, title)

	// Collapse multiple hyphens
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}

	// Trim leading/trailing hyphens
	result = strings.Trim(result, "-")

	if result == "" {
		return "Report"
	}

	// Limit length
	if len(result) > 30 {
		result = result[:30]
		result = strings.TrimRight(result, "-")
	}

	return result
}

// loadDisclaimer loads disclaimer content from the configured path.
// Path format: "playbook-name/path/to/file.md" or "none" for no disclaimer.
// Returns empty string if "none" or if file not found.
func (s *Service) loadDisclaimer(disclaimerPath string) string {
	// Handle "none" explicitly - no disclaimer
	if disclaimerPath == "none" || disclaimerPath == "" {
		return ""
	}

	// Parse path: "playbook-name/path/to/file.md"
	parts := strings.SplitN(disclaimerPath, "/", 2)
	if len(parts) < 2 {
		s.logger.Warnf("Invalid disclaimer path format (expected playbook/path): %s", disclaimerPath)
		return ""
	}

	playbookName := parts[0]
	filePath := parts[1]

	// Build full path (playbook files are stored directly under playbook root, not in a "files" subdir)
	fullPath := filepath.Join(s.config.PlaybooksDir(), playbookName, filePath)

	// Read file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		s.logger.Warnf("Failed to load disclaimer from %s: %v", fullPath, err)
		return ""
	}

	return string(content)
}
