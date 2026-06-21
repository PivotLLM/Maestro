/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/reporting"
)

// handleTaskRun handles the task_run MCP tool
func (p *Provider) handleTaskRun(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	taskType := parseString(call.Args, "type", "")
	parallelStr := parseString(call.Args, "parallel", "")

	p.logToolCall(global.ToolTaskRun, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	// Build run request - parallel is optional override
	runReq := &global.RunRequest{
		Project: project,
		Path:    path,
		Type:    taskType,
	}

	// Only set Parallel if explicitly provided
	if parallelStr != "" {
		parallelVal := parallelStr == "true"
		runReq.Parallel = &parallelVal
	}

	result, err := p.runner.Run(call.Ctx, runReq, completionSink(call))
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to run tasks: %v", err)), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleTaskStatus handles the task_status MCP tool
func (p *Provider) handleTaskStatus(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	taskType := parseString(call.Args, "type", "")

	p.logToolCall(global.ToolTaskStatus, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	result, err := p.runner.GetTaskStatus(project, path, taskType)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get task status: %v", err)), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleTaskResults handles the task_results MCP tool
func (p *Provider) handleTaskResults(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	status := parseString(call.Args, "status", "")
	taskID := int(parseFloat64(call.Args, "task_id", -1))
	offset := int(parseFloat64(call.Args, "offset", 0))
	limit := int(parseFloat64(call.Args, "limit", float64(global.DefaultLimit)))
	summary := parseBool(call.Args, "summary", false)
	workerPattern := parseString(call.Args, "worker_pattern", "")
	qaPattern := parseString(call.Args, "qa_pattern", "")

	p.logToolCall(global.ToolTaskResults, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	resultsReq := &global.ResultsRequest{
		Project:       project,
		Path:          path,
		Status:        status,
		Offset:        offset,
		Limit:         limit,
		Summary:       summary,
		WorkerPattern: workerPattern,
		QAPattern:     qaPattern,
	}

	// Check if single task requested
	if taskID >= 0 {
		resultsReq.TaskID = &taskID
	}

	result, err := p.runner.GetResults(resultsReq)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get results: %v", err)), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleTaskResultGet handles the task_result_get MCP tool
// Returns a single task result with just the worker/QA responses (no history or prompts)
func (p *Provider) handleTaskResultGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	uuid := parseString(call.Args, "uuid", "")

	p.logToolCall(global.ToolTaskResultGet, map[string]string{"project": project, "uuid": uuid})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if uuid == "" {
		return nil, fmt.Errorf("%s", "uuid is required")
	}

	// Get task to find the taskset path and template
	task, taskPath, err := p.tasks.GetTask(project, uuid)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get task: %v", err)), IsError: true}, nil
	}

	// Get taskset to retrieve template info
	taskset, err := p.tasks.GetTaskSet(project, taskPath)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get taskset: %v", err)), IsError: true}, nil
	}

	// Load the actual schema content if template is specified
	var schemaContent string
	if taskset.WorkerResponseTemplate != "" {
		if content, err := p.loadTemplate(project, taskset.WorkerResponseTemplate); err == nil {
			schemaContent = content
		}
		// If loading fails, we just leave schemaContent empty - not critical
	}

	// Load result file
	resultsDir := p.tasks.GetResultsDir(project)
	resultPath := filepath.Join(resultsDir, uuid+".json")

	data, err := os.ReadFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Task exists but no result yet - return basic info with empty responses
			response := global.TaskResultGetResponse{
				TaskID:                 task.ID,
				TaskUUID:               task.UUID,
				TaskTitle:              task.Title,
				TaskType:               task.Type,
				TaskPath:               taskPath,
				WorkerResponseTemplate: taskset.WorkerResponseTemplate,
				WorkerResponseSchema:   schemaContent,
				WorkerStatus:           task.Work.Status,
				WorkerError:            task.Work.Error,
				WorkerErrorCode:        task.Work.ErrorCode,
				QAEnabled:              task.QA.Enabled,
			}
			return createJSONResult(response)
		}
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to read result file: %v", err)), IsError: true}, nil
	}

	var taskResult global.TaskResult
	if err := json.Unmarshal(data, &taskResult); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to parse result file: %v", err)), IsError: true}, nil
	}

	// Build condensed response
	response := global.TaskResultGetResponse{
		TaskID:                 taskResult.TaskID,
		TaskUUID:               taskResult.TaskUUID,
		TaskTitle:              taskResult.TaskTitle,
		TaskType:               taskResult.TaskType,
		TaskPath:               taskPath,
		WorkerResponseTemplate: taskset.WorkerResponseTemplate,
		WorkerResponseSchema:   schemaContent,
		WorkerStatus:           taskResult.Worker.Status,
		WorkerResponse:         taskResult.Worker.Response,
		WorkerError:            taskResult.Worker.Error,
		WorkerErrorCode:        taskResult.Worker.ErrorCode,
		QAEnabled:              task.QA.Enabled,
		SupervisorOverride:     taskResult.SupervisorOverride,
		CompletedAt:            taskResult.CompletedAt,
	}

	// Add QA info if available
	if taskResult.QA != nil {
		response.QAStatus = taskResult.QA.Status
		response.QAVerdict = taskResult.QA.Verdict
		response.QAResponse = taskResult.QA.Response
		response.QAError = taskResult.QA.Error
	}

	return createJSONResult(response)
}

// handleTaskReport handles the task_report MCP tool
func (p *Provider) handleTaskReport(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	status := parseString(call.Args, "status", "")
	taskType := parseString(call.Args, "type", "")
	qaVerdict := parseString(call.Args, "qa_verdict", "")
	format := parseString(call.Args, "format", "markdown")
	outputPath := parseString(call.Args, "output", "")

	p.logToolCall(global.ToolTaskReport, map[string]string{"project": project, "format": format})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	// Build filter
	var filter *reporting.ReportFilter
	if path != "" || status != "" || qaVerdict != "" || taskType != "" {
		filter = &reporting.ReportFilter{
			PathPrefix:   path,
			StatusFilter: status,
			QAVerdict:    qaVerdict,
		}

		// Handle type filter
		if taskType != "" {
			filter.Types = []string{taskType}
		}
	}

	// List all task sets for the project
	taskSetList, err := p.tasks.ListTaskSets(project, path)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to list task sets: %v", err)), IsError: true}, nil
	}

	// Create content loaders for template loading
	playbookLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid playbook path: %s (expected playbook-name/path)", path)
		}
		item, err := p.playbooks.GetFile(parts[0], parts[1], 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	referenceLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		item, err := p.reference.Get(path, 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	projectLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		item, err := p.projects.GetFile(project, path, 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	// Get results directory for loading task results
	resultsDir := p.tasks.GetResultsDir(project)

	// Build and generate report
	reporter := reporting.New(p.logger,
		reporting.WithPlaybookLoader(playbookLoader),
		reporting.WithReferenceLoader(referenceLoader),
		reporting.WithProjectLoader(projectLoader),
	)
	report := reporter.BuildReport(project, taskSetList.TaskSets, filter, resultsDir)

	// Generate report in requested format
	var content string
	switch format {
	case "json":
		content, err = reporter.GenerateJSON(report)
	case "markdown", "md":
		content, err = reporter.GenerateHierarchicalMarkdown(report)
	default:
		content, err = reporter.GenerateHierarchicalMarkdown(report)
	}

	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to generate report: %v", err)), IsError: true}, nil
	}

	// Optionally save to file in project files directory
	if outputPath != "" {
		if _, err := p.projects.PutFile(project, outputPath, content, "Generated report"); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to save report: %v", err)), IsError: true}, nil
		}
	}

	return &toolspec.Result{ForLLM: content}, nil
}
