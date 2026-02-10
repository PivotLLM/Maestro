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
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/PivotLLM/Maestro/global"
)

// handleSupervisorUpdate handles the supervisor_update MCP tool.
// Allows a supervisor to replace the worker response with their own content.
// The response must pass template validation. History is append-only.
func (s *Server) handleSupervisorUpdate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	uuid := mcp.ParseString(request, "uuid", "")
	response := mcp.ParseString(request, "response", "")

	s.logToolCall(global.ToolSupervisorUpdate, map[string]string{"project": project, "uuid": uuid})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if uuid == "" {
		return mcp.NewToolResultError("uuid parameter is required"), nil
	}
	if response == "" {
		return mcp.NewToolResultError("response parameter is required"), nil
	}

	// Get task to find the taskset for template validation
	task, taskPath, err := s.tasks.GetTask(project, uuid)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get task: %v", err)), nil
	}

	// Get taskset for template
	taskset, err := s.tasks.GetTaskSet(project, taskPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get taskset: %v", err)), nil
	}

	// Validate response against worker_response_template
	if taskset.WorkerResponseTemplate != "" {
		// Load template
		templateContent, err := s.loadTemplate(project, taskset.WorkerResponseTemplate)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to load response template: %v", err)), nil
		}

		// Parse template as JSON schema
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(templateContent), &schema); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse response template: %v", err)), nil
		}

		// Parse response as JSON
		var responseData map[string]interface{}
		if err := json.Unmarshal([]byte(response), &responseData); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("response must be valid JSON matching template. Template:\n%s\n\nYour response is not valid JSON: %v", templateContent, err)), nil
		}

		// Basic validation: check required fields exist
		if err := validateResponseAgainstSchema(responseData, schema); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("response does not match template. Template:\n%s\n\nValidation error: %v", templateContent, err)), nil
		}
	}

	// Load existing result
	resultsDir := s.tasks.GetResultsDir(project)
	resultPath := filepath.Join(resultsDir, uuid+".json")

	var taskResult global.TaskResult
	resultData, err := os.ReadFile(resultPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create new result if it doesn't exist
			taskResult = global.TaskResult{
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
					LLMModelID:             task.Work.LLMModelID,
				},
				History: []global.Message{},
			}
		} else {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read result file: %v", err)), nil
		}
	} else {
		if err := json.Unmarshal(resultData, &taskResult); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to parse result file: %v", err)), nil
		}
	}

	// Add supervisor message to history
	supervisorMessage := global.Message{
		Timestamp: time.Now(),
		Role:      "supervisor",
		Stdout:    response,
	}
	taskResult.History = append(taskResult.History, supervisorMessage)

	// Update worker response
	taskResult.Worker.Response = response
	taskResult.Worker.Status = global.ExecutionStatusDone
	taskResult.Worker.Error = ""

	// Clear QA data and mark as superseded (supervisor override invalidates prior QA)
	if taskResult.QA != nil {
		taskResult.QA.Response = ""
		taskResult.QA.Verdict = ""
		taskResult.QA.Error = ""
		taskResult.QA.FullPrompt = ""
		taskResult.QA.Status = "superseded"
	}

	// Set supervisor override flag
	taskResult.SupervisorOverride = true
	taskResult.CompletedAt = time.Now()

	// Save result file
	newResultData, err := json.MarshalIndent(taskResult, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}

	if err := os.WriteFile(resultPath, newResultData, 0644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save result: %v", err)), nil
	}

	// Update task status to done and clear QA verdict
	updates := map[string]interface{}{
		"work": map[string]interface{}{
			"status": global.ExecutionStatusDone,
		},
		"qa": map[string]interface{}{
			"verdict": "N/A",
			"status":  "superseded",
		},
	}
	if _, err := s.tasks.UpdateTask(project, uuid, updates); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update task status: %v", err)), nil
	}

	result := map[string]interface{}{
		"project":             project,
		"uuid":                uuid,
		"task_id":             task.ID,
		"supervisor_override": true,
		"status":              "done",
		"message":             "Supervisor response applied successfully",
	}

	return createJSONResult(result)
}

// loadTemplate loads a template file from playbook or project files
func (s *Server) loadTemplate(project, templatePath string) (string, error) {
	// Try playbook first (format: playbook-name/path/to/file)
	parts := strings.SplitN(templatePath, "/", 2)
	if len(parts) >= 2 {
		playbookName := parts[0]
		filePath := parts[1]
		fullPath := filepath.Join(s.config.PlaybooksDir(), playbookName, filePath)
		if content, err := os.ReadFile(fullPath); err == nil {
			return string(content), nil
		}
	}

	// Try project files
	proj, err := s.projects.Get(project)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %w", err)
	}
	_ = proj // Could use for project-specific file lookup

	return "", fmt.Errorf("template not found: %s", templatePath)
}

// validateResponseAgainstSchema performs basic validation of response against JSON schema
func validateResponseAgainstSchema(response, schema map[string]interface{}) error {
	// Get required fields from schema
	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		// No properties defined, accept any valid JSON
		return nil
	}

	required, _ := schema["required"].([]interface{})
	requiredMap := make(map[string]bool)
	for _, r := range required {
		if s, ok := r.(string); ok {
			requiredMap[s] = true
		}
	}

	// Check all required fields are present
	for field := range requiredMap {
		if _, exists := response[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// Check all response fields are defined in schema
	for field := range response {
		if _, exists := properties[field]; !exists {
			return fmt.Errorf("unexpected field: %s", field)
		}
	}

	return nil
}

// handleReportCreate handles the report_create MCP tool.
// Generates reports from task results using the same logic as the runner.
func (s *Server) handleReportCreate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")

	s.logToolCall(global.ToolReportCreate, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	// Get project to retrieve its title for the report session
	proj, err := s.projects.Get(project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to get project: %v", err)), nil
	}

	// Start a new report session with a fresh prefix
	_, err = s.projects.StartReport(project, proj.Title, "")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to start report session: %v", err)), nil
	}

	// Use runner's GenerateReport function
	reports, err := s.runner.GenerateReport(project, path)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to generate report: %v", err)), nil
	}

	result := map[string]interface{}{
		"project":       project,
		"reports":       reports,
		"reports_count": len(reports),
		"message":       fmt.Sprintf("Generated %d report(s)", len(reports)),
	}

	return createJSONResult(result)
}
