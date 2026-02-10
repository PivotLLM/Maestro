/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/reporting"
	"github.com/mark3labs/mcp-go/mcp"
)

// handleTaskRun handles the task_run MCP tool
func (s *Server) handleTaskRun(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	taskType := mcp.ParseString(req, "type", "")
	parallelStr := mcp.ParseString(req, "parallel", "")
	timeout := int(mcp.ParseFloat64(req, "timeout", 0))
	wait := mcp.ParseBoolean(req, "wait", false)

	s.logToolCall(global.ToolTaskRun, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	// Build run request - parallel is optional override
	runReq := &global.RunRequest{
		Project: project,
		Path:    path,
		Type:    taskType,
		Timeout: timeout,
		Wait:    wait,
	}

	// Only set Parallel if explicitly provided
	if parallelStr != "" {
		parallelVal := parallelStr == "true"
		runReq.Parallel = &parallelVal
	}

	result, err := s.runner.Run(ctx, runReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to run tasks: %v", err)), nil
	}

	return createJSONResult(result)
}

// handleTaskStatus handles the task_status MCP tool
func (s *Server) handleTaskStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	taskType := mcp.ParseString(req, "type", "")

	s.logToolCall(global.ToolTaskStatus, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	result, err := s.runner.GetTaskStatus(project, path, taskType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get task status: %v", err)), nil
	}

	return createJSONResult(result)
}

// handleTaskResults handles the task_results MCP tool
func (s *Server) handleTaskResults(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	status := mcp.ParseString(req, "status", "")
	taskID := int(mcp.ParseFloat64(req, "task_id", -1))
	offset := int(mcp.ParseFloat64(req, "offset", 0))
	limit := int(mcp.ParseFloat64(req, "limit", float64(global.DefaultLimit)))
	summary := mcp.ParseBoolean(req, "summary", false)
	workerPattern := mcp.ParseString(req, "worker_pattern", "")
	qaPattern := mcp.ParseString(req, "qa_pattern", "")

	s.logToolCall(global.ToolTaskResults, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
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

	result, err := s.runner.GetResults(resultsReq)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get results: %v", err)), nil
	}

	return createJSONResult(result)
}

// handleTaskResultGet handles the task_result_get MCP tool
// Returns a single task result with just the worker/QA responses (no history or prompts)
func (s *Server) handleTaskResultGet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	uuid := mcp.ParseString(req, "uuid", "")

	s.logToolCall(global.ToolTaskResultGet, map[string]string{"project": project, "uuid": uuid})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if uuid == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}

	// Get task to find the taskset path and template
	task, taskPath, err := s.tasks.GetTask(project, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get task: %v", err)), nil
	}

	// Get taskset to retrieve template info
	taskset, err := s.tasks.GetTaskSet(project, taskPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get taskset: %v", err)), nil
	}

	// Load the actual schema content if template is specified
	var schemaContent string
	if taskset.WorkerResponseTemplate != "" {
		if content, err := s.loadTemplate(project, taskset.WorkerResponseTemplate); err == nil {
			schemaContent = content
		}
		// If loading fails, we just leave schemaContent empty - not critical
	}

	// Load result file
	resultsDir := s.tasks.GetResultsDir(project)
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
				QAEnabled:              task.QA.Enabled,
			}
			return createJSONResult(response)
		}
		return mcp.NewToolResultError(fmt.Sprintf("failed to read result file: %v", err)), nil
	}

	var taskResult global.TaskResult
	if err := json.Unmarshal(data, &taskResult); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to parse result file: %v", err)), nil
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
func (s *Server) handleTaskReport(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	status := mcp.ParseString(req, "status", "")
	taskType := mcp.ParseString(req, "type", "")
	qaVerdict := mcp.ParseString(req, "qa_verdict", "")
	format := mcp.ParseString(req, "format", "markdown")
	outputPath := mcp.ParseString(req, "output", "")

	s.logToolCall(global.ToolTaskReport, map[string]string{"project": project, "format": format})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
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
	taskSetList, err := s.tasks.ListTaskSets(project, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list task sets: %v", err)), nil
	}

	// Create content loaders for template loading
	playbookLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid playbook path: %s (expected playbook-name/path)", path)
		}
		item, err := s.playbooks.GetFile(parts[0], parts[1], 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	referenceLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		item, err := s.reference.Get(path, 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	projectLoader := reporting.ContentLoaderFunc(func(path string) (string, error) {
		item, err := s.projects.GetFile(project, path, 0, 0)
		if err != nil {
			return "", err
		}
		return item.Content, nil
	})

	// Get results directory for loading task results
	resultsDir := s.tasks.GetResultsDir(project)

	// Build and generate report
	reporter := reporting.New(s.logger,
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
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate report: %v", err)), nil
	}

	// Optionally save to file in project files directory
	if outputPath != "" {
		if _, err := s.projects.PutFile(project, outputPath, content, "Generated report"); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to save report: %v", err)), nil
		}
	}

	return mcp.NewToolResultText(content), nil
}
