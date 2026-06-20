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
	"time"

	"github.com/PivotLLM/Maestro/global"
)

// handleSupervisorUpdate handles the supervisor_update MCP tool.
// Allows a supervisor to replace the worker response with their own content.
// The response must pass template validation. History is append-only.
func (p *Provider) handleSupervisorUpdate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	uuid := parseString(call.Args, "uuid", "")
	response := parseString(call.Args, "response", "")

	p.logToolCall(global.ToolSupervisorUpdate, map[string]string{"project": project, "uuid": uuid})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if uuid == "" {
		return nil, fmt.Errorf("%s", "uuid parameter is required")
	}
	if response == "" {
		return nil, fmt.Errorf("%s", "response parameter is required")
	}

	// Get task to find the taskset for template validation
	task, taskPath, err := p.tasks.GetTask(project, uuid)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get task: %v", err)), IsError: true}, nil
	}

	// Get taskset for template
	taskset, err := p.tasks.GetTaskSet(project, taskPath)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get taskset: %v", err)), IsError: true}, nil
	}

	// Validate response against worker_response_template
	if taskset.WorkerResponseTemplate != "" {
		// Load template
		templateContent, err := p.loadTemplate(project, taskset.WorkerResponseTemplate)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to load response template: %v", err)), IsError: true}, nil
		}

		// Parse template as JSON schema
		var schema map[string]interface{}
		if err := json.Unmarshal([]byte(templateContent), &schema); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to parse response template: %v", err)), IsError: true}, nil
		}

		// Parse response as JSON
		var responseData map[string]interface{}
		if err := json.Unmarshal([]byte(response), &responseData); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("response must be valid JSON matching template. Template:\n%s\n\nYour response is not valid JSON: %v", templateContent, err)), IsError: true}, nil
		}

		// Basic validation: check required fields exist
		if err := validateResponseAgainstSchema(responseData, schema); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("response does not match template. Template:\n%s\n\nValidation error: %v", templateContent, err)), IsError: true}, nil
		}
	}

	// Load existing result
	resultsDir := p.tasks.GetResultsDir(project)
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
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to read result file: %v", err)), IsError: true}, nil
		}
	} else {
		if err := json.Unmarshal(resultData, &taskResult); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to parse result file: %v", err)), IsError: true}, nil
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
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to marshal result: %v", err)), IsError: true}, nil
	}

	if err := os.WriteFile(resultPath, newResultData, 0644); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to save result: %v", err)), IsError: true}, nil
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
	if _, err := p.tasks.UpdateTask(project, uuid, updates); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to update task status: %v", err)), IsError: true}, nil
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
func (p *Provider) loadTemplate(project, templatePath string) (string, error) {
	// Try playbook first (format: playbook-name/path/to/file)
	parts := strings.SplitN(templatePath, "/", 2)
	if len(parts) >= 2 {
		playbookName := parts[0]
		filePath := parts[1]
		fullPath := filepath.Join(p.config.PlaybooksDir(), playbookName, filePath)
		if content, err := os.ReadFile(fullPath); err == nil {
			return string(content), nil
		}
	}

	// Try project files
	proj, err := p.projects.Get(project)
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
func (p *Provider) handleReportCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolReportCreate, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

	// Get project to retrieve its title for the report session
	proj, err := p.projects.Get(project)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to get project: %v", err)), IsError: true}, nil
	}

	// Start a new report session with a fresh prefix
	_, err = p.projects.StartReport(project, proj.Title, "")
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to start report session: %v", err)), IsError: true}, nil
	}

	// Use runner's GenerateReport function
	reports, err := p.runner.GenerateReport(project, path)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to generate report: %v", err)), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project":       project,
		"reports":       reports,
		"reports_count": len(reports),
		"message":       fmt.Sprintf("Generated %d report(s)", len(reports)),
	}

	return createJSONResult(result)
}
