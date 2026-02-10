/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package reporting provides hierarchical report generation from task results.
package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

// ContentLoader loads content from a specific source (playbook, project, reference)
type ContentLoader interface {
	GetContent(path string) (string, error)
}

// ContentLoaderFunc is a function type that implements ContentLoader
type ContentLoaderFunc func(path string) (string, error)

// GetContent implements ContentLoader
func (f ContentLoaderFunc) GetContent(path string) (string, error) {
	return f(path)
}

// Reporter generates reports from task results
type Reporter struct {
	logger          *logging.Logger
	projectLoader   ContentLoader
	playbookLoader  ContentLoader // accepts paths in format "playbook-name/path/to/file"
	referenceLoader ContentLoader
	templateCache   map[string]*template.Template
}

// Option configures a Reporter
type Option func(*Reporter)

// WithProjectLoader sets the project content loader
func WithProjectLoader(loader ContentLoader) Option {
	return func(r *Reporter) {
		r.projectLoader = loader
	}
}

// WithPlaybookLoader sets the playbook content loader.
// The loader receives paths in format "playbook-name/path/to/file".
func WithPlaybookLoader(loader ContentLoader) Option {
	return func(r *Reporter) {
		r.playbookLoader = loader
	}
}

// WithReferenceLoader sets the reference content loader
func WithReferenceLoader(loader ContentLoader) Option {
	return func(r *Reporter) {
		r.referenceLoader = loader
	}
}

// New creates a new Reporter
func New(logger *logging.Logger, opts ...Option) *Reporter {
	r := &Reporter{
		logger:        logger,
		templateCache: make(map[string]*template.Template),
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

// loadTemplate loads and parses a template from the specified source
func (r *Reporter) loadTemplate(templatePath, source string) (*template.Template, error) {
	cacheKey := source + ":" + templatePath
	if tmpl, ok := r.templateCache[cacheKey]; ok {
		return tmpl, nil
	}

	var content string
	var err error

	switch source {
	case "project":
		if r.projectLoader == nil {
			return nil, fmt.Errorf("project loader not configured")
		}
		content, err = r.projectLoader.GetContent(templatePath)
	case "playbook":
		// Path format: "playbook-name/path/to/template.md"
		if r.playbookLoader == nil {
			return nil, fmt.Errorf("playbook loader not configured")
		}
		content, err = r.playbookLoader.GetContent(templatePath)
	case "reference":
		if r.referenceLoader == nil {
			return nil, fmt.Errorf("reference loader not configured")
		}
		content, err = r.referenceLoader.GetContent(templatePath)
	default:
		return nil, fmt.Errorf("unknown template source: %s", source)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load template: %w", err)
	}

	tmpl, err := template.New(templatePath).Funcs(templateFuncs()).Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	r.templateCache[cacheKey] = tmpl
	return tmpl, nil
}

// templateFuncs returns custom template functions
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": strings.Title,
		"json": func(v interface{}) string {
			data, err := json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Sprintf("error: %v", err)
			}
			return string(data)
		},
	}
}

// LoadTemplateConfigs loads report template configurations from a template path.
// If the path ends in .json, it's parsed as a manifest containing multiple templates.
// If the path ends in .md, it's treated as a single template with suffix "Report".
// Returns nil if the template path is empty or loading fails.
func (r *Reporter) LoadTemplateConfigs(templatePath string) []global.ReportTemplateConfig {
	if templatePath == "" {
		return nil
	}

	// Single .md template - backwards compatible
	if strings.HasSuffix(templatePath, ".md") {
		return []global.ReportTemplateConfig{
			{Suffix: "Report", File: templatePath},
		}
	}

	// JSON manifest - load and parse
	if strings.HasSuffix(templatePath, ".json") {
		configs := r.loadTemplateManifest(templatePath)
		if configs != nil {
			return configs
		}
	}

	// Unknown extension or load failed - treat as single template
	return []global.ReportTemplateConfig{
		{Suffix: "Report", File: templatePath},
	}
}

// loadTemplateManifest loads a JSON template manifest file
func (r *Reporter) loadTemplateManifest(manifestPath string) []global.ReportTemplateConfig {
	var content string
	var err error

	// Try playbook loader first if path has components
	if r.playbookLoader != nil && strings.Contains(manifestPath, "/") {
		content, err = r.playbookLoader.GetContent(manifestPath)
		if err != nil && r.logger != nil {
			r.logger.Debugf("Failed to load manifest from playbook: %v", err)
		}
	}

	// Fall back to project loader
	if content == "" && r.projectLoader != nil {
		content, err = r.projectLoader.GetContent(manifestPath)
		if err != nil {
			if r.logger != nil {
				r.logger.Warnf("Failed to load template manifest %s: %v", manifestPath, err)
			}
			return nil
		}
	}

	if content == "" {
		return nil
	}

	// Parse the manifest
	var configs []global.ReportTemplateConfig
	if err := json.Unmarshal([]byte(content), &configs); err != nil {
		if r.logger != nil {
			r.logger.Warnf("Failed to parse template manifest %s: %v", manifestPath, err)
		}
		return nil
	}

	// Resolve relative paths - files are relative to the manifest location
	manifestDir := filepath.Dir(manifestPath)
	for i := range configs {
		if !strings.HasPrefix(configs[i].File, "/") && !strings.Contains(configs[i].File, "/") {
			// Relative path - prepend manifest directory
			configs[i].File = filepath.Join(manifestDir, configs[i].File)
		}
	}

	return configs
}

// RenderWithTemplate renders a task result using the configured template
// It determines the template source based on path format:
// - Paths in format "playbook-name/path/file.md" are loaded from playbooks
// - Other paths are loaded from the project
// If playbook loading fails, it falls back to project loading.
func (r *Reporter) RenderWithTemplate(task TaskReport, templatePath string) string {
	if templatePath == "" {
		return task.WorkResult
	}

	// Convention: paths with at least two components (playbook-name/path) try playbook first
	// if the playbook loader is configured
	if r.playbookLoader != nil && strings.Contains(templatePath, "/") {
		result := r.renderTaskResult(task, templatePath, "playbook")
		if result != task.WorkResult {
			return result
		}
		// Playbook loading failed, try project
	}

	return r.renderTaskResult(task, templatePath, "project")
}

// RenderQAWithTemplate renders a QA result using the configured QA template
// It follows the same path resolution as RenderWithTemplate.
func (r *Reporter) RenderQAWithTemplate(task TaskReport, templatePath string) string {
	if templatePath == "" || task.QAResult == "" {
		return task.QAResult
	}

	// Convention: paths with at least two components (playbook-name/path) try playbook first
	if r.playbookLoader != nil && strings.Contains(templatePath, "/") {
		result := r.renderQAResult(task, templatePath, "playbook")
		if result != task.QAResult {
			return result
		}
		// Playbook loading failed, try project
	}

	return r.renderQAResult(task, templatePath, "project")
}

// renderQAResult renders a QA result using its template or returns raw result
func (r *Reporter) renderQAResult(task TaskReport, templatePath, templateSource string) string {
	if templatePath == "" {
		return task.QAResult
	}

	// Try to parse the QA result as JSON for template data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(task.QAResult), &data); err != nil {
		// Not valid JSON, return raw result
		if r.logger != nil {
			r.logger.Debugf("Task %d: QA result is not JSON, using raw output", task.ID)
		}
		return task.QAResult
	}

	// Add task metadata to the data for templates that need it
	data["_task_id"] = task.ID
	data["_task_title"] = task.Title
	data["_task_type"] = task.Type
	data["_task_status"] = task.WorkStatus
	data["_qa_verdict"] = task.QAVerdict

	// Load and execute template
	tmpl, err := r.loadTemplate(templatePath, templateSource)
	if err != nil {
		if r.logger != nil {
			r.logger.Warnf("Task %d: Failed to load QA template %s: %v", task.ID, templatePath, err)
		}
		return task.QAResult
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		if r.logger != nil {
			r.logger.Warnf("Task %d: Failed to execute QA template: %v", task.ID, err)
		}
		return task.QAResult
	}

	return buf.String()
}

// renderTaskResult renders a task result using its template or returns raw result
func (r *Reporter) renderTaskResult(task TaskReport, templatePath, templateSource string) string {
	// If no template specified, return raw result
	if templatePath == "" {
		return task.WorkResult
	}

	// Try to parse the work result as JSON for template data
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(task.WorkResult), &data); err != nil {
		// Not valid JSON, return raw result
		if r.logger != nil {
			r.logger.Debugf("Task %d: Work result is not JSON, using raw output", task.ID)
		}
		return task.WorkResult
	}

	// Add task metadata to the data for templates that need it
	data["_task_id"] = task.ID
	data["_task_title"] = task.Title
	data["_task_type"] = task.Type
	data["_task_status"] = task.WorkStatus
	data["_qa_verdict"] = task.QAVerdict

	// Add QA result as parsed JSON for template access
	if task.QAResult != "" {
		var qaData map[string]interface{}
		if err := json.Unmarshal([]byte(task.QAResult), &qaData); err == nil {
			data["_qa_result"] = qaData
		}
	}

	// Load and execute template
	tmpl, err := r.loadTemplate(templatePath, templateSource)
	if err != nil {
		if r.logger != nil {
			r.logger.Warnf("Task %d: Failed to load template %s: %v", task.ID, templatePath, err)
		}
		return task.WorkResult
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		if r.logger != nil {
			r.logger.Warnf("Task %d: Failed to execute template: %v", task.ID, err)
		}
		return task.WorkResult
	}

	return buf.String()
}

// ProjectReport represents a complete project report
type ProjectReport struct {
	Project     string          `json:"project"`
	Title       string          `json:"title"`
	GeneratedAt time.Time       `json:"generated_at"`
	Summary     ReportSummary   `json:"summary"`
	TaskSets    []TaskSetReport `json:"task_sets"`
}

// ReportSummary contains aggregate statistics
type ReportSummary struct {
	TotalTasks       int            `json:"total_tasks"`
	CompletedTasks   int            `json:"completed_tasks"`
	FailedTasks      int            `json:"failed_tasks"`
	PendingTasks     int            `json:"pending_tasks"`
	QAPassedTasks    int            `json:"qa_passed_tasks"`
	QAFailedTasks    int            `json:"qa_failed_tasks"`
	QAEscalatedTasks int            `json:"qa_escalated_tasks"`
	ByVerdict        map[string]int `json:"by_verdict,omitempty"`
	ByType           map[string]int `json:"by_type,omitempty"`
}

// TaskSetReport represents a task set in the report
type TaskSetReport struct {
	Path                 string       `json:"path"`
	Title                string       `json:"title"`
	Description          string       `json:"description,omitempty"`
	WorkerReportTemplate string       `json:"worker_report_template,omitempty"`
	QAReportTemplate     string       `json:"qa_report_template,omitempty"`
	Tasks                []TaskReport `json:"tasks"`
}

// TaskReport represents a task in the report
type TaskReport struct {
	ID          int        `json:"id"`
	UUID        string     `json:"uuid"`
	Title       string     `json:"title"`
	Type        string     `json:"type"`
	WorkStatus  string     `json:"work_status"`
	WorkResult  string     `json:"work_result,omitempty"`
	QAEnabled   bool       `json:"qa_enabled"`
	QAVerdict   string     `json:"qa_verdict,omitempty"` // "pass", "fail", "escalate"
	QAFeedback  string     `json:"qa_feedback,omitempty"`
	QAIssues    []string   `json:"qa_issues,omitempty"`
	QAResult    string     `json:"qa_result,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ReportFilter specifies filters for report generation
type ReportFilter struct {
	PathPrefix   string   // Filter by task set path prefix
	StatusFilter string   // Filter by work status (done, failed, etc.)
	QAVerdict    string   // Filter by QA verdict (pass, fail, escalate)
	Types        []string // Filter by task types
}

// BuildReport builds a report from task sets
// resultsDir is the path to the results directory; if empty, results won't be loaded
func (r *Reporter) BuildReport(project string, taskSets []*global.TaskSet, filter *ReportFilter, resultsDir string) *ProjectReport {
	report := &ProjectReport{
		Project:     project,
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			ByVerdict: make(map[string]int),
			ByType:    make(map[string]int),
		},
	}

	for _, ts := range taskSets {
		// Apply path filter
		if filter != nil && filter.PathPrefix != "" {
			if !strings.HasPrefix(ts.Path, filter.PathPrefix) {
				continue
			}
		}

		taskSetReport := TaskSetReport{
			Path:                 ts.Path,
			Title:                ts.Title,
			Description:          ts.Description,
			WorkerReportTemplate: ts.WorkerReportTemplate,
			QAReportTemplate:     ts.QAReportTemplate,
		}

		for _, task := range ts.Tasks {
			// Apply status filter
			if filter != nil && filter.StatusFilter != "" {
				if task.Work.Status != filter.StatusFilter {
					continue
				}
			}

			// Apply QA verdict filter
			if filter != nil && filter.QAVerdict != "" {
				if task.QA.Enabled && task.QA.Verdict != filter.QAVerdict {
					continue
				}
			}

			// Apply type filter
			if filter != nil && len(filter.Types) > 0 {
				found := false
				for _, t := range filter.Types {
					if task.Type == t {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			taskReport := TaskReport{
				ID:         task.ID,
				UUID:       task.UUID,
				Title:      task.Title,
				Type:       task.Type,
				WorkStatus: task.Work.Status,
				QAEnabled:  task.QA.Enabled,
			}

			// Load results from results file if available
			if resultsDir != "" && (task.Work.Status == global.ExecutionStatusDone || task.Work.Status == global.ExecutionStatusFailed) {
				resultPath := filepath.Join(resultsDir, task.UUID+".json")
				if data, err := os.ReadFile(resultPath); err == nil {
					var result global.TaskResult
					if err := json.Unmarshal(data, &result); err == nil {
						taskReport.WorkResult = result.Worker.Response
						if result.QA != nil {
							taskReport.QAResult = result.QA.Response
						}
					}
				}
			}

			if task.QA.Enabled {
				taskReport.QAVerdict = task.QA.Verdict
				// Extract feedback/notes/comments and issues from QA result if loaded
				if taskReport.QAResult != "" {
					var qaResult struct {
						Feedback string   `json:"feedback"`
						Notes    string   `json:"notes"`
						Comments string   `json:"comments"`
						Issues   []string `json:"issues"`
					}
					if err := json.Unmarshal([]byte(taskReport.QAResult), &qaResult); err == nil {
						// Use feedback if present, otherwise notes, otherwise comments
						if qaResult.Feedback != "" {
							taskReport.QAFeedback = qaResult.Feedback
						} else if qaResult.Notes != "" {
							taskReport.QAFeedback = qaResult.Notes
						} else if qaResult.Comments != "" {
							taskReport.QAFeedback = qaResult.Comments
						}
						taskReport.QAIssues = qaResult.Issues
					}
				}
			}

			taskSetReport.Tasks = append(taskSetReport.Tasks, taskReport)

			// Update summary
			report.Summary.TotalTasks++
			report.Summary.ByType[task.Type]++

			switch task.Work.Status {
			case global.ExecutionStatusDone:
				report.Summary.CompletedTasks++
			case global.ExecutionStatusFailed:
				report.Summary.FailedTasks++
			default:
				report.Summary.PendingTasks++
			}

			// Update QA verdict counts
			if task.QA.Enabled && task.QA.Verdict != "" {
				report.Summary.ByVerdict[task.QA.Verdict]++
				switch task.QA.Verdict {
				case global.QAVerdictPass:
					report.Summary.QAPassedTasks++
				case global.QAVerdictFail:
					report.Summary.QAFailedTasks++
				case global.QAVerdictEscalate:
					report.Summary.QAEscalatedTasks++
				}
			}
		}

		if len(taskSetReport.Tasks) > 0 {
			report.TaskSets = append(report.TaskSets, taskSetReport)
		}
	}

	return report
}

// GenerateMarkdown generates a markdown report
func (r *Reporter) GenerateMarkdown(report *ProjectReport) (string, error) {
	tmpl := `# Project Report: {{.Project}}

**Generated**: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}

## Summary

| Metric | Count |
|--------|-------|
| Total Tasks | {{.Summary.TotalTasks}} |
| Completed | {{.Summary.CompletedTasks}} |
| Failed | {{.Summary.FailedTasks}} |
| Pending | {{.Summary.PendingTasks}} |
{{if gt .Summary.QAPassedTasks 0}}| QA Passed | {{.Summary.QAPassedTasks}} |{{end}}
{{if gt .Summary.QAFailedTasks 0}}| QA Failed | {{.Summary.QAFailedTasks}} |{{end}}
{{if gt .Summary.QAEscalatedTasks 0}}| QA Escalated | {{.Summary.QAEscalatedTasks}} |{{end}}

{{if .Summary.ByVerdict}}
### By Verdict

| Verdict | Count |
|---------|-------|
{{range $k, $v := .Summary.ByVerdict}}| {{$k}} | {{$v}} |
{{end}}{{end}}

{{if .Summary.ByType}}
### By Type

| Type | Count |
|------|-------|
{{range $k, $v := .Summary.ByType}}| {{$k}} | {{$v}} |
{{end}}{{end}}

---

{{range .TaskSets}}
## {{.Title}}

**Path**: ` + "`{{.Path}}`" + `
{{if .Description}}
{{.Description}}
{{end}}

{{range .Tasks}}
### Task {{.ID}}: {{.Title}}

- **Type**: {{.Type}}
- **Status**: {{.WorkStatus}}
{{if .QAEnabled}}- **QA**: {{.QAVerdict}}{{end}}

{{if .WorkResult}}
#### Result

{{.WorkResult}}
{{end}}

{{if and .QAEnabled .QAResult}}
#### QA Review

{{if .QAFeedback}}{{.QAFeedback}}{{else}}{{.QAResult}}{{end}}
{{if .QAIssues}}
**Issues**:
{{range .QAIssues}}- {{.}}
{{end}}{{end}}
{{end}}

---

{{end}}
{{end}}
`

	t, err := template.New("report").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, report); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// GenerateHierarchicalMarkdown generates a hierarchical markdown report organized by path
func (r *Reporter) GenerateHierarchicalMarkdown(report *ProjectReport) (string, error) {
	// Build path hierarchy
	hierarchy := make(map[string][]TaskSetReport)

	for _, ts := range report.TaskSets {
		segments := strings.Split(ts.Path, "/")
		prefix := ""
		if len(segments) > 0 {
			prefix = segments[0]
		}
		hierarchy[prefix] = append(hierarchy[prefix], ts)
	}

	// Sort prefixes
	var prefixes []string
	for prefix := range hierarchy {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Project Report: %s\n\n", report.Project))
	sb.WriteString(fmt.Sprintf("**Generated**: %s\n\n", report.GeneratedAt.Format("2006-01-02 15:04:05")))

	// Summary
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total Tasks**: %d\n", report.Summary.TotalTasks))
	sb.WriteString(fmt.Sprintf("- **Completed**: %d\n", report.Summary.CompletedTasks))
	sb.WriteString(fmt.Sprintf("- **Failed**: %d\n", report.Summary.FailedTasks))
	sb.WriteString(fmt.Sprintf("- **Pending**: %d\n", report.Summary.PendingTasks))

	if report.Summary.QAPassedTasks > 0 || report.Summary.QAFailedTasks > 0 {
		sb.WriteString(fmt.Sprintf("- **QA Passed**: %d\n", report.Summary.QAPassedTasks))
		sb.WriteString(fmt.Sprintf("- **QA Failed**: %d\n", report.Summary.QAFailedTasks))
	}

	sb.WriteString("\n---\n\n")

	// Hierarchical content
	for _, prefix := range prefixes {
		taskSets := hierarchy[prefix]

		// Check if this is a single task set at the prefix level (no sub-paths)
		// If so, just use the task set title as H2 without a redundant H3
		singleFlatTaskSet := len(taskSets) == 1 && taskSets[0].Path == prefix

		if !singleFlatTaskSet && prefix != "" {
			sb.WriteString(fmt.Sprintf("## %s\n\n", strings.Title(prefix)))
		}

		for _, ts := range taskSets {
			if singleFlatTaskSet {
				// Use task set title as H2 directly
				sb.WriteString(fmt.Sprintf("## %s\n\n", ts.Title))
			} else {
				sb.WriteString(fmt.Sprintf("### %s\n\n", ts.Title))
			}

			if ts.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n", ts.Description))
			}

			sb.WriteString("\n")

			for _, task := range ts.Tasks {
				sb.WriteString(fmt.Sprintf("### %s\n\n", task.Title))
				sb.WriteString(fmt.Sprintf("**Task**: %d\n", task.ID))
				sb.WriteString(fmt.Sprintf("**Status**: %s\n", task.WorkStatus))

				if task.QAEnabled {
					switch task.QAVerdict {
					case global.QAVerdictPass:
						sb.WriteString("**QA**: Pass\n")
					case global.QAVerdictFail:
						sb.WriteString("**QA**: Fail\n")
					case global.QAVerdictEscalate:
						sb.WriteString("**QA**: Escalate\n")
					default:
						sb.WriteString(fmt.Sprintf("**QA**: %s\n", task.QAVerdict))
					}
				} else {
					sb.WriteString("**QA**: None\n")
				}

				if task.WorkResult != "" {
					sb.WriteString("\n")
					// Use template if configured, otherwise raw result
					renderedResult := r.RenderWithTemplate(task, ts.WorkerReportTemplate)
					sb.WriteString(renderedResult)
					sb.WriteString("\n")
				}

				// Show QA results for all QA-enabled tasks (not just failures)
				if task.QAEnabled && task.QAResult != "" {
					sb.WriteString("\n**QA Review**\n\n")
					// Use template if configured, otherwise use raw result or feedback
					if ts.QAReportTemplate != "" {
						renderedQA := r.RenderQAWithTemplate(task, ts.QAReportTemplate)
						sb.WriteString(renderedQA)
					} else if task.QAFeedback != "" {
						sb.WriteString(task.QAFeedback)
					} else {
						sb.WriteString(task.QAResult)
					}
					sb.WriteString("\n")

					// Show issues list if present
					if len(task.QAIssues) > 0 {
						sb.WriteString("\n**Issues**:\n\n")
						for _, issue := range task.QAIssues {
							sb.WriteString(fmt.Sprintf("- %s\n", issue))
						}
					}
				}

				sb.WriteString("\n---\n\n")
			}
		}
	}

	return sb.String(), nil
}

// GenerateJSON generates a JSON report
func (r *Reporter) GenerateJSON(report *ProjectReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal report: %w", err)
	}
	return string(data), nil
}

// SaveReport saves a report to a file
func (r *Reporter) SaveReport(report *ProjectReport, outputPath, format string) error {
	var content string
	var err error

	switch format {
	case "markdown", "md":
		content, err = r.GenerateHierarchicalMarkdown(report)
	case "json":
		content, err = r.GenerateJSON(report)
	default:
		content, err = r.GenerateHierarchicalMarkdown(report)
	}

	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	if r.logger != nil {
		r.logger.Infof("Report saved to %s (%d bytes)", outputPath, len(content))
	}

	return nil
}

// GenerateFilename generates a timestamped report filename
func GenerateFilename(prefix string, format string) string {
	timestamp := time.Now().Format("2006-01-02-150405")
	ext := "md"
	if format == "json" {
		ext = "json"
	}

	if prefix == "" {
		prefix = "report"
	}

	return fmt.Sprintf("%s-%s.%s", prefix, timestamp, ext)
}
