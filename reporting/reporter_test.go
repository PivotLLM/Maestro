/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package reporting

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/global"
)

func TestNew(t *testing.T) {
	r := New(nil)
	if r == nil {
		t.Fatal("New returned nil")
	}
}

func TestBuildReport(t *testing.T) {
	r := New(nil)

	taskSets := []*global.TaskSet{
		{
			Path:        "security/scanning",
			Title:       "Security Scanning",
			Description: "Security analysis tasks",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-1",
					Title: "Scan dependencies",
					Type:  "analysis",
					Work: global.WorkExecution{
						Status: global.ExecutionStatusDone,
					},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictPass,
						Status:  global.ExecutionStatusDone,
					},
				},
				{
					ID:    2,
					UUID:  "uuid-2",
					Title: "Scan code",
					Type:  "analysis",
					Work: global.WorkExecution{
						Status: global.ExecutionStatusFailed,
					},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictFail,
						Status:  global.ExecutionStatusDone,
					},
				},
			},
		},
		{
			Path:        "docs/review",
			Title:       "Documentation Review",
			Description: "Review documentation",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-3",
					Title: "Review README",
					Type:  "review",
					Work: global.WorkExecution{
						Status: global.ExecutionStatusWaiting,
					},
				},
			},
		},
	}

	report := r.BuildReport("test-project", taskSets, nil, "")

	if report.Project != "test-project" {
		t.Errorf("expected project test-project, got %s", report.Project)
	}

	if report.Summary.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks, got %d", report.Summary.TotalTasks)
	}

	if report.Summary.CompletedTasks != 1 {
		t.Errorf("expected 1 completed task, got %d", report.Summary.CompletedTasks)
	}

	if report.Summary.FailedTasks != 1 {
		t.Errorf("expected 1 failed task, got %d", report.Summary.FailedTasks)
	}

	if report.Summary.PendingTasks != 1 {
		t.Errorf("expected 1 pending task, got %d", report.Summary.PendingTasks)
	}

	if report.Summary.QAPassedTasks != 1 {
		t.Errorf("expected 1 QA passed task, got %d", report.Summary.QAPassedTasks)
	}

	if report.Summary.QAFailedTasks != 1 {
		t.Errorf("expected 1 QA failed task, got %d", report.Summary.QAFailedTasks)
	}

	if len(report.TaskSets) != 2 {
		t.Errorf("expected 2 task sets, got %d", len(report.TaskSets))
	}
}

func TestBuildReportWithFilter(t *testing.T) {
	r := New(nil)

	taskSets := []*global.TaskSet{
		{
			Path:  "security/scanning",
			Title: "Security Scanning",
			Tasks: []global.Task{
				{
					ID:   1,
					Type: "analysis",
					Work: global.WorkExecution{Status: global.ExecutionStatusDone},
				},
				{
					ID:   2,
					Type: "review",
					Work: global.WorkExecution{Status: global.ExecutionStatusFailed},
				},
			},
		},
		{
			Path:  "docs/review",
			Title: "Documentation Review",
			Tasks: []global.Task{
				{
					ID:   1,
					Type: "review",
					Work: global.WorkExecution{Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	// Test path filter
	filter := &ReportFilter{PathPrefix: "security"}
	report := r.BuildReport("test", taskSets, filter, "")
	if len(report.TaskSets) != 1 {
		t.Errorf("path filter: expected 1 task set, got %d", len(report.TaskSets))
	}

	// Test status filter
	filter = &ReportFilter{StatusFilter: global.ExecutionStatusDone}
	report = r.BuildReport("test", taskSets, filter, "")
	if report.Summary.TotalTasks != 2 {
		t.Errorf("status filter: expected 2 tasks, got %d", report.Summary.TotalTasks)
	}

	// Test type filter
	filter = &ReportFilter{Types: []string{"review"}}
	report = r.BuildReport("test", taskSets, filter, "")
	if report.Summary.TotalTasks != 2 {
		t.Errorf("type filter: expected 2 tasks, got %d", report.Summary.TotalTasks)
	}
}

func TestBuildReportQAFilter(t *testing.T) {
	r := New(nil)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:   1,
					Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictPass,
						Status:  global.ExecutionStatusDone,
					},
				},
				{
					ID:   2,
					Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictFail,
						Status:  global.ExecutionStatusDone,
					},
				},
				{
					ID:   3,
					Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictEscalate,
						Status:  global.ExecutionStatusDone,
					},
				},
			},
		},
	}

	// Filter QA verdict = pass
	filter := &ReportFilter{QAVerdict: global.QAVerdictPass}
	report := r.BuildReport("test", taskSets, filter, "")
	if report.Summary.TotalTasks != 1 {
		t.Errorf("QA pass filter: expected 1 task, got %d", report.Summary.TotalTasks)
	}

	// Filter QA verdict = fail
	filter = &ReportFilter{QAVerdict: global.QAVerdictFail}
	report = r.BuildReport("test", taskSets, filter, "")
	if report.Summary.TotalTasks != 1 {
		t.Errorf("QA fail filter: expected 1 task, got %d", report.Summary.TotalTasks)
	}

	// Filter QA verdict = escalate
	filter = &ReportFilter{QAVerdict: global.QAVerdictEscalate}
	report = r.BuildReport("test", taskSets, filter, "")
	if report.Summary.TotalTasks != 1 {
		t.Errorf("QA escalate filter: expected 1 task, got %d", report.Summary.TotalTasks)
	}
}

func TestGenerateMarkdown(t *testing.T) {
	r := New(nil)

	report := &ProjectReport{
		Project:     "test-project",
		GeneratedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
		Summary: ReportSummary{
			TotalTasks:     2,
			CompletedTasks: 1,
			FailedTasks:    1,
			ByVerdict:      map[string]int{global.QAVerdictFail: 1},
			ByType:         map[string]int{"analysis": 2},
		},
		TaskSets: []TaskSetReport{
			{
				Path:        "security/scan",
				Title:       "Security Scan",
				Description: "Security analysis",
				Tasks: []TaskReport{
					{
						ID:         1,
						Title:      "Scan code",
						Type:       "analysis",
						WorkStatus: global.ExecutionStatusDone,
						WorkResult: "Found issues",
						QAEnabled:  true,
						QAVerdict:  global.QAVerdictFail,
						QAFeedback: "Quality issues",
					},
				},
			},
		},
	}

	md, err := r.GenerateMarkdown(report)
	if err != nil {
		t.Fatalf("GenerateMarkdown failed: %v", err)
	}

	// Check key content
	if !strings.Contains(md, "test-project") {
		t.Error("markdown should contain project name")
	}
	if !strings.Contains(md, "Security Scan") {
		t.Error("markdown should contain task set title")
	}
	if !strings.Contains(md, "Scan code") {
		t.Error("markdown should contain task title")
	}
}

func TestGenerateHierarchicalMarkdown(t *testing.T) {
	r := New(nil)

	report := &ProjectReport{
		Project:     "test-project",
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			TotalTasks:     2,
			CompletedTasks: 2,
		},
		TaskSets: []TaskSetReport{
			{
				Path:  "security/scanning",
				Title: "Security Scanning",
				Tasks: []TaskReport{
					{ID: 1, Title: "Task 1", WorkStatus: global.ExecutionStatusDone},
				},
			},
			{
				Path:  "security/review",
				Title: "Security Review",
				Tasks: []TaskReport{
					{ID: 1, Title: "Task 2", WorkStatus: global.ExecutionStatusDone},
				},
			},
		},
	}

	md, err := r.GenerateHierarchicalMarkdown(report)
	if err != nil {
		t.Fatalf("GenerateHierarchicalMarkdown failed: %v", err)
	}

	// Check hierarchical structure
	if !strings.Contains(md, "## Security") {
		t.Error("markdown should contain security section header")
	}
	if !strings.Contains(md, "### Security Scanning") {
		t.Error("markdown should contain task set headers")
	}
}

func TestGenerateJSON(t *testing.T) {
	r := New(nil)

	report := &ProjectReport{
		Project:     "test-project",
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			TotalTasks: 1,
		},
		TaskSets: []TaskSetReport{
			{
				Path:  "test",
				Title: "Test",
				Tasks: []TaskReport{
					{ID: 1, Title: "Task 1"},
				},
			},
		},
	}

	jsonStr, err := r.GenerateJSON(report)
	if err != nil {
		t.Fatalf("GenerateJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed ProjectReport
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed.Project != "test-project" {
		t.Errorf("expected project test-project, got %s", parsed.Project)
	}
}

func TestSaveReport(t *testing.T) {
	r := New(nil)

	report := &ProjectReport{
		Project:     "test-project",
		GeneratedAt: time.Now(),
		Summary:     ReportSummary{TotalTasks: 1},
		TaskSets: []TaskSetReport{
			{Path: "test", Title: "Test", Tasks: []TaskReport{{ID: 1, Title: "Task"}}},
		},
	}

	tmpDir := t.TempDir()

	// Test markdown save
	mdPath := filepath.Join(tmpDir, "report.md")
	if err := r.SaveReport(report, mdPath, "markdown"); err != nil {
		t.Fatalf("SaveReport markdown failed: %v", err)
	}

	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		t.Error("markdown report file not created")
	}

	// Test JSON save
	jsonPath := filepath.Join(tmpDir, "report.json")
	if err := r.SaveReport(report, jsonPath, "json"); err != nil {
		t.Fatalf("SaveReport json failed: %v", err)
	}

	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Error("JSON report file not created")
	}

	// Test subdirectory creation
	nestedPath := filepath.Join(tmpDir, "nested", "dir", "report.md")
	if err := r.SaveReport(report, nestedPath, "md"); err != nil {
		t.Fatalf("SaveReport nested failed: %v", err)
	}

	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("nested report file not created")
	}
}

func TestGenerateFilename(t *testing.T) {
	tests := []struct {
		prefix  string
		format  string
		wantExt string
		wantPre string
	}{
		{"project", "markdown", "md", "project"},
		{"project", "json", "json", "project"},
		{"", "md", "md", "report"},
		{"custom", "other", "md", "custom"},
	}

	for _, tt := range tests {
		filename := GenerateFilename(tt.prefix, tt.format)

		if !strings.HasPrefix(filename, tt.wantPre+"-") {
			t.Errorf("GenerateFilename(%q, %q) = %q, want prefix %q", tt.prefix, tt.format, filename, tt.wantPre)
		}

		if !strings.HasSuffix(filename, "."+tt.wantExt) {
			t.Errorf("GenerateFilename(%q, %q) = %q, want extension %q", tt.prefix, tt.format, filename, tt.wantExt)
		}
	}
}

func TestBuildReportEmptyTaskSets(t *testing.T) {
	r := New(nil)

	report := r.BuildReport("empty-project", []*global.TaskSet{}, nil, "")

	if report.Summary.TotalTasks != 0 {
		t.Errorf("expected 0 tasks, got %d", report.Summary.TotalTasks)
	}

	if len(report.TaskSets) != 0 {
		t.Errorf("expected 0 task sets, got %d", len(report.TaskSets))
	}
}

func TestBuildReportFilterRemovesEmptyTaskSets(t *testing.T) {
	r := New(nil)

	taskSets := []*global.TaskSet{
		{
			Path:  "include",
			Title: "Include",
			Tasks: []global.Task{
				{ID: 1, Work: global.WorkExecution{Status: global.ExecutionStatusDone}},
			},
		},
		{
			Path:  "exclude",
			Title: "Exclude",
			Tasks: []global.Task{
				{ID: 1, Work: global.WorkExecution{Status: global.ExecutionStatusFailed}},
			},
		},
	}

	// Filter only done tasks - should remove exclude task set
	filter := &ReportFilter{StatusFilter: global.ExecutionStatusDone}
	report := r.BuildReport("test", taskSets, filter, "")

	if len(report.TaskSets) != 1 {
		t.Errorf("expected 1 task set after filter, got %d", len(report.TaskSets))
	}

	if report.TaskSets[0].Path != "include" {
		t.Errorf("expected include task set, got %s", report.TaskSets[0].Path)
	}
}

// ============================================================================
// QA Template Rendering Tests
// ============================================================================

func TestRenderQAWithTemplate(t *testing.T) {
	// Create a mock loader that returns template content
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		if path == "qa-template.md" {
			return "**Verdict**: {{.qa_verdict}}\n**Notes**: {{.notes}}", nil
		}
		return "", os.ErrNotExist
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:        1,
		Title:     "Test Task",
		QAEnabled: true,
		QAVerdict: "pass",
		QAResult:  `{"qa_verdict": "Pass", "notes": "Good work"}`,
	}

	result := r.RenderQAWithTemplate(task, "qa-template.md")

	if !strings.Contains(result, "**Verdict**: Pass") {
		t.Errorf("expected rendered template with verdict, got: %s", result)
	}
	if !strings.Contains(result, "**Notes**: Good work") {
		t.Errorf("expected rendered template with notes, got: %s", result)
	}
}

func TestRenderQAWithTemplateEmptyPath(t *testing.T) {
	r := New(nil)

	task := TaskReport{
		ID:       1,
		QAResult: `{"qa_verdict": "Pass"}`,
	}

	// Empty template path should return raw result
	result := r.RenderQAWithTemplate(task, "")
	if result != task.QAResult {
		t.Errorf("expected raw QA result, got: %s", result)
	}
}

func TestRenderQAWithTemplateEmptyQAResult(t *testing.T) {
	r := New(nil)

	task := TaskReport{
		ID:       1,
		QAResult: "",
	}

	// Empty QA result should return empty string
	result := r.RenderQAWithTemplate(task, "template.md")
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}

func TestRenderQAWithTemplateNonJSONResult(t *testing.T) {
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		return "Template: {{.data}}", nil
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:       1,
		QAResult: "This is plain text, not JSON",
	}

	// Non-JSON result should return raw result
	result := r.RenderQAWithTemplate(task, "template.md")
	if result != task.QAResult {
		t.Errorf("expected raw QA result for non-JSON, got: %s", result)
	}
}

func TestRenderQAWithTemplatePlaybookFallback(t *testing.T) {
	playbookLoader := ContentLoaderFunc(func(path string) (string, error) {
		if path == "playbook/qa-template.md" {
			return "Playbook: {{.verdict}}", nil
		}
		return "", os.ErrNotExist
	})

	projectLoader := ContentLoaderFunc(func(path string) (string, error) {
		return "Project: {{.verdict}}", nil
	})

	r := New(nil, WithPlaybookLoader(playbookLoader), WithProjectLoader(projectLoader))

	task := TaskReport{
		ID:       1,
		QAResult: `{"verdict": "Pass"}`,
	}

	// Should use playbook template when path has components
	result := r.RenderQAWithTemplate(task, "playbook/qa-template.md")
	if !strings.Contains(result, "Playbook:") {
		t.Errorf("expected playbook template to be used, got: %s", result)
	}
}

// ============================================================================
// Worker Template Rendering Tests
// ============================================================================

func TestRenderWithTemplate(t *testing.T) {
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		if path == "worker-template.md" {
			return "# {{._task_title}}\n\nResult: {{.finding}}", nil
		}
		return "", os.ErrNotExist
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:         1,
		Title:      "Security Scan",
		Type:       "analysis",
		WorkStatus: "done",
		WorkResult: `{"finding": "No vulnerabilities found"}`,
	}

	result := r.RenderWithTemplate(task, "worker-template.md")

	if !strings.Contains(result, "# Security Scan") {
		t.Errorf("expected task title in rendered template, got: %s", result)
	}
	if !strings.Contains(result, "No vulnerabilities found") {
		t.Errorf("expected finding in rendered template, got: %s", result)
	}
}

func TestRenderWithTemplateEmptyPath(t *testing.T) {
	r := New(nil)

	task := TaskReport{
		ID:         1,
		WorkResult: "Raw work result",
	}

	result := r.RenderWithTemplate(task, "")
	if result != task.WorkResult {
		t.Errorf("expected raw work result, got: %s", result)
	}
}

func TestRenderWithTemplateIncludesQAResult(t *testing.T) {
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		return "QA Verdict: {{._qa_result.verdict}}", nil
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:         1,
		WorkResult: `{"data": "test"}`,
		QAResult:   `{"verdict": "Pass"}`,
	}

	result := r.RenderWithTemplate(task, "template.md")
	if !strings.Contains(result, "QA Verdict: Pass") {
		t.Errorf("expected QA result accessible in worker template, got: %s", result)
	}
}

// ============================================================================
// Template Loading and Caching Tests
// ============================================================================

func TestTemplateCaching(t *testing.T) {
	callCount := 0
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		callCount++
		return "Template content: {{.value}}", nil
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:         1,
		WorkResult: `{"value": "test"}`,
	}

	// Call multiple times with same template
	r.RenderWithTemplate(task, "cached-template.md")
	r.RenderWithTemplate(task, "cached-template.md")
	r.RenderWithTemplate(task, "cached-template.md")

	// Template should only be loaded once due to caching
	if callCount != 1 {
		t.Errorf("expected template to be loaded once (cached), but was loaded %d times", callCount)
	}
}

func TestTemplateCachingDifferentSources(t *testing.T) {
	projectCallCount := 0
	playbookCallCount := 0

	projectLoader := ContentLoaderFunc(func(path string) (string, error) {
		projectCallCount++
		return "Project: {{.value}}", nil
	})

	playbookLoader := ContentLoaderFunc(func(path string) (string, error) {
		playbookCallCount++
		return "Playbook: {{.value}}", nil
	})

	r := New(nil, WithProjectLoader(projectLoader), WithPlaybookLoader(playbookLoader))

	task := TaskReport{
		ID:         1,
		WorkResult: `{"value": "test"}`,
	}

	// Same path but different sources should be cached separately
	r.RenderWithTemplate(task, "template.md")               // project
	r.RenderWithTemplate(task, "playbook-name/template.md") // playbook

	if projectCallCount != 1 {
		t.Errorf("expected project loader called once, got %d", projectCallCount)
	}
	if playbookCallCount != 1 {
		t.Errorf("expected playbook loader called once, got %d", playbookCallCount)
	}
}

func TestLoadTemplateNoLoaderConfigured(t *testing.T) {
	r := New(nil) // No loaders configured

	task := TaskReport{
		ID:         1,
		WorkResult: `{"value": "test"}`,
	}

	// Should return raw result when no loader is configured
	result := r.RenderWithTemplate(task, "template.md")
	if result != task.WorkResult {
		t.Errorf("expected raw result when loader not configured, got: %s", result)
	}
}

// ============================================================================
// Notes/Feedback/Comments Extraction Tests
// ============================================================================

func TestBuildReportQAFeedbackExtraction(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with feedback
	resultData := global.TaskResult{
		Worker: global.WorkerResult{
			Response: "Worker output",
		},
		QA: &global.QAResult{
			Response: `{"qa_verdict": "Pass", "feedback": "Great work on this task"}`,
		},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-feedback.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-feedback",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{
						Enabled: true,
						Verdict: global.QAVerdictPass,
						Status:  global.ExecutionStatusDone,
					},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)

	if len(report.TaskSets) != 1 || len(report.TaskSets[0].Tasks) != 1 {
		t.Fatal("expected 1 task set with 1 task")
	}

	task := report.TaskSets[0].Tasks[0]
	if task.QAFeedback != "Great work on this task" {
		t.Errorf("expected QAFeedback to be extracted, got: %s", task.QAFeedback)
	}
}

func TestBuildReportQANotesExtraction(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with notes (instead of feedback)
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA:     &global.QAResult{Response: `{"qa_verdict": "Pass", "notes": "Some notes here"}`},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-notes.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-notes",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if task.QAFeedback != "Some notes here" {
		t.Errorf("expected notes to be extracted as QAFeedback, got: %s", task.QAFeedback)
	}
}

func TestBuildReportQACommentsExtraction(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with comments (fallback when no feedback/notes)
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA:     &global.QAResult{Response: `{"qa_verdict": "Pass", "comments": "Reviewer comments"}`},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-comments.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-comments",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if task.QAFeedback != "Reviewer comments" {
		t.Errorf("expected comments to be extracted as QAFeedback, got: %s", task.QAFeedback)
	}
}

func TestBuildReportQAFeedbackPriority(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with all three: feedback, notes, comments
	// Feedback should take priority
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA: &global.QAResult{
			Response: `{"qa_verdict": "Pass", "feedback": "Primary feedback", "notes": "Secondary notes", "comments": "Tertiary comments"}`,
		},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-priority.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-priority",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if task.QAFeedback != "Primary feedback" {
		t.Errorf("expected feedback to take priority, got: %s", task.QAFeedback)
	}
}

// ============================================================================
// Issues Array Handling Tests
// ============================================================================

func TestBuildReportQAIssuesExtraction(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with issues array
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA: &global.QAResult{
			Response: `{"qa_verdict": "Fail", "issues": ["Missing error handling", "No unit tests", "Poor documentation"]}`,
		},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-issues.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-issues",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictFail, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if len(task.QAIssues) != 3 {
		t.Errorf("expected 3 issues, got %d", len(task.QAIssues))
	}

	expectedIssues := []string{"Missing error handling", "No unit tests", "Poor documentation"}
	for i, expected := range expectedIssues {
		if task.QAIssues[i] != expected {
			t.Errorf("issue %d: expected %q, got %q", i, expected, task.QAIssues[i])
		}
	}
}

func TestBuildReportQAIssuesEmptyArray(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file with empty issues array
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA:     &global.QAResult{Response: `{"qa_verdict": "Pass", "issues": []}`},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-empty-issues.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-empty-issues",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if len(task.QAIssues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(task.QAIssues))
	}
}

func TestBuildReportQANoIssuesField(t *testing.T) {
	r := New(nil)
	tmpDir := t.TempDir()
	resultsDir := tmpDir

	// Create a task result file without issues field
	resultData := global.TaskResult{
		Worker: global.WorkerResult{Response: "Worker output"},
		QA:     &global.QAResult{Response: `{"qa_verdict": "Pass", "feedback": "Looks good"}`},
	}
	resultBytes, _ := json.Marshal(resultData)
	os.WriteFile(filepath.Join(resultsDir, "uuid-no-issues.json"), resultBytes, 0644)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{
					ID:    1,
					UUID:  "uuid-no-issues",
					Title: "Test Task",
					Work:  global.WorkExecution{Status: global.ExecutionStatusDone},
					QA:    global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone},
				},
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, resultsDir)
	task := report.TaskSets[0].Tasks[0]

	if task.QAIssues != nil && len(task.QAIssues) != 0 {
		t.Errorf("expected nil or empty issues, got %v", task.QAIssues)
	}
}

// ============================================================================
// ContentLoader Options Tests
// ============================================================================

func TestWithProjectLoader(t *testing.T) {
	called := false
	loader := ContentLoaderFunc(func(path string) (string, error) {
		called = true
		return "content", nil
	})

	r := New(nil, WithProjectLoader(loader))

	task := TaskReport{ID: 1, WorkResult: `{"x": 1}`}
	r.RenderWithTemplate(task, "test.md")

	if !called {
		t.Error("expected project loader to be called")
	}
}

func TestWithPlaybookLoader(t *testing.T) {
	called := false
	loader := ContentLoaderFunc(func(path string) (string, error) {
		called = true
		return "content", nil
	})

	r := New(nil, WithPlaybookLoader(loader))

	task := TaskReport{ID: 1, WorkResult: `{"x": 1}`}
	r.RenderWithTemplate(task, "playbook/test.md")

	if !called {
		t.Error("expected playbook loader to be called")
	}
}

func TestWithReferenceLoader(t *testing.T) {
	// Reference loader is used for loadTemplate when source is "reference"
	// This is typically used internally, but we can test the option works
	loader := ContentLoaderFunc(func(path string) (string, error) {
		return "reference content", nil
	})

	r := New(nil, WithReferenceLoader(loader))

	if r.referenceLoader == nil {
		t.Error("expected reference loader to be set")
	}
}

// ============================================================================
// Template Functions Tests
// ============================================================================

func TestTemplateFunctions(t *testing.T) {
	mockLoader := ContentLoaderFunc(func(path string) (string, error) {
		return `Upper: {{upper .text}} | Lower: {{lower .text}} | JSON: {{json .obj}}`, nil
	})

	r := New(nil, WithProjectLoader(mockLoader))

	task := TaskReport{
		ID:         1,
		WorkResult: `{"text": "Hello", "obj": {"key": "value"}}`,
	}

	result := r.RenderWithTemplate(task, "template.md")

	if !strings.Contains(result, "Upper: HELLO") {
		t.Errorf("expected upper function to work, got: %s", result)
	}
	if !strings.Contains(result, "Lower: hello") {
		t.Errorf("expected lower function to work, got: %s", result)
	}
	if !strings.Contains(result, `"key": "value"`) {
		t.Errorf("expected json function to work, got: %s", result)
	}
}

// ============================================================================
// Report Verdict Summary Tests
// ============================================================================

func TestBuildReportVerdictSummary(t *testing.T) {
	r := New(nil)

	taskSets := []*global.TaskSet{
		{
			Path:  "test",
			Title: "Test",
			Tasks: []global.Task{
				{ID: 1, Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone}},
				{ID: 2, Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{Enabled: true, Verdict: global.QAVerdictPass, Status: global.ExecutionStatusDone}},
				{ID: 3, Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{Enabled: true, Verdict: global.QAVerdictFail, Status: global.ExecutionStatusDone}},
				{ID: 4, Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{Enabled: true, Verdict: global.QAVerdictEscalate, Status: global.ExecutionStatusDone}},
				{ID: 5, Work: global.WorkExecution{Status: global.ExecutionStatusDone},
					QA: global.QAExecution{Enabled: false}}, // No QA
			},
		},
	}

	report := r.BuildReport("test", taskSets, nil, "")

	if report.Summary.QAPassedTasks != 2 {
		t.Errorf("expected 2 passed, got %d", report.Summary.QAPassedTasks)
	}
	if report.Summary.QAFailedTasks != 1 {
		t.Errorf("expected 1 failed, got %d", report.Summary.QAFailedTasks)
	}
	if report.Summary.QAEscalatedTasks != 1 {
		t.Errorf("expected 1 escalated, got %d", report.Summary.QAEscalatedTasks)
	}

	// Check ByVerdict map
	if report.Summary.ByVerdict[global.QAVerdictPass] != 2 {
		t.Errorf("expected ByVerdict[pass]=2, got %d", report.Summary.ByVerdict[global.QAVerdictPass])
	}
	if report.Summary.ByVerdict[global.QAVerdictFail] != 1 {
		t.Errorf("expected ByVerdict[fail]=1, got %d", report.Summary.ByVerdict[global.QAVerdictFail])
	}
	if report.Summary.ByVerdict[global.QAVerdictEscalate] != 1 {
		t.Errorf("expected ByVerdict[escalate]=1, got %d", report.Summary.ByVerdict[global.QAVerdictEscalate])
	}
}
