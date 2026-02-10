/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/PivotLLM/Maestro/global"
	templatespkg "github.com/PivotLLM/Maestro/templates"
	"github.com/mark3labs/mcp-go/mcp"
)

// handleTaskSetCreate handles the taskset_create MCP tool
func (s *Server) handleTaskSetCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	title := mcp.ParseString(req, "title", "")
	description := mcp.ParseString(req, "description", "")
	parallel := mcp.ParseBoolean(req, "parallel", false)
	maxWorker := int(mcp.ParseFloat64(req, "max_worker", 0))
	maxQA := int(mcp.ParseFloat64(req, "max_qa", 0))
	workerResponseTemplate := mcp.ParseString(req, "worker_response_template", "")
	workerReportTemplate := mcp.ParseString(req, "worker_report_template", "")
	qaResponseTemplate := mcp.ParseString(req, "qa_response_template", "")
	qaReportTemplate := mcp.ParseString(req, "qa_report_template", "")

	s.logToolCall(global.ToolTaskSetCreate, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	// Validate limits if provided
	var limits global.Limits
	if maxWorker > 0 {
		validated, err := global.ValidateMaxWorker(maxWorker)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limits.MaxWorker = validated
	}
	if maxQA > 0 {
		validated, err := global.ValidateMaxQA(maxQA)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limits.MaxQA = validated
	}

	// Build templates if any are provided
	var templates *global.DefaultTemplates
	if workerResponseTemplate != "" || workerReportTemplate != "" || qaResponseTemplate != "" || qaReportTemplate != "" {
		templates = &global.DefaultTemplates{
			WorkerResponseTemplate: workerResponseTemplate,
			WorkerReportTemplate:   workerReportTemplate,
			QAResponseTemplate:     qaResponseTemplate,
			QAReportTemplate:       qaReportTemplate,
		}
	}

	// Validate QA response schema if provided
	if qaResponseTemplate != "" {
		schemaContent := s.loadSchemaContent(qaResponseTemplate)
		if schemaContent != "" {
			if err := templatespkg.ValidateQASchema(schemaContent); err != nil {
				return mcp.NewToolResultError("invalid qa_response_template: " + err.Error()), nil
			}
		}
	}

	taskSet, err := s.tasks.CreateTaskSet(project, path, title, description, templates, parallel, limits)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetGet handles the taskset_get MCP tool
func (s *Server) handleTaskSetGet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")

	s.logToolCall(global.ToolTaskSetGet, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	taskSet, err := s.tasks.GetTaskSet(project, path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetList handles the taskset_list MCP tool
func (s *Server) handleTaskSetList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	pathPrefix := mcp.ParseString(req, "path", "")

	s.logToolCall(global.ToolTaskSetList, map[string]string{"project": project, "path": pathPrefix})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	result, err := s.tasks.ListTaskSets(project, pathPrefix)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

// handleTaskSetUpdate handles the taskset_update MCP tool
func (s *Server) handleTaskSetUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	titleStr := mcp.ParseString(req, "title", "")
	descriptionStr := mcp.ParseString(req, "description", "")
	parallelStr := mcp.ParseString(req, "parallel", "")
	maxWorkerVal := int(mcp.ParseFloat64(req, "max_worker", -1))
	maxQAVal := int(mcp.ParseFloat64(req, "max_qa", -1))
	workerResponseTemplate := mcp.ParseString(req, "worker_response_template", "")
	workerReportTemplate := mcp.ParseString(req, "worker_report_template", "")
	qaResponseTemplate := mcp.ParseString(req, "qa_response_template", "")
	qaReportTemplate := mcp.ParseString(req, "qa_report_template", "")

	s.logToolCall(global.ToolTaskSetUpdate, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	var title, description *string
	var parallel *bool
	var limits *global.Limits
	if titleStr != "" {
		title = &titleStr
	}
	if descriptionStr != "" {
		description = &descriptionStr
	}
	if parallelStr != "" {
		parallelVal := parallelStr == "true"
		parallel = &parallelVal
	}

	// Handle limits updates
	if maxWorkerVal >= 0 || maxQAVal >= 0 {
		limits = &global.Limits{}
		if maxWorkerVal >= 0 {
			validated, err := global.ValidateMaxWorker(maxWorkerVal)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			limits.MaxWorker = validated
		}
		if maxQAVal >= 0 {
			validated, err := global.ValidateMaxQA(maxQAVal)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			limits.MaxQA = validated
		}
	}

	// Build templates if any are provided
	var templates *global.DefaultTemplates
	if workerResponseTemplate != "" || workerReportTemplate != "" || qaResponseTemplate != "" || qaReportTemplate != "" {
		templates = &global.DefaultTemplates{
			WorkerResponseTemplate: workerResponseTemplate,
			WorkerReportTemplate:   workerReportTemplate,
			QAResponseTemplate:     qaResponseTemplate,
			QAReportTemplate:       qaReportTemplate,
		}
	}

	// Validate QA response schema if being updated
	if qaResponseTemplate != "" {
		schemaContent := s.loadSchemaContent(qaResponseTemplate)
		if schemaContent != "" {
			if err := templatespkg.ValidateQASchema(schemaContent); err != nil {
				return mcp.NewToolResultError("invalid qa_response_template: " + err.Error()), nil
			}
		}
	}

	taskSet, err := s.tasks.UpdateTaskSet(project, path, title, description, templates, parallel, limits)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetDelete handles the taskset_delete MCP tool
func (s *Server) handleTaskSetDelete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")

	s.logToolCall(global.ToolTaskSetDelete, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}

	if err := s.tasks.DeleteTaskSet(project, path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"deleted": true,
	}

	return createJSONResult(result)
}

// handleTaskSetReset handles the taskset_reset MCP tool
func (s *Server) handleTaskSetReset(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	mode := mcp.ParseString(req, "mode", "")
	deleteResults := mcp.ParseBoolean(req, "delete_results", true)
	endReport := mcp.ParseBoolean(req, "end_report", false)

	s.logToolCall(global.ToolTaskSetReset, map[string]string{"project": project, "path": path, "mode": mode})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	if mode == "" {
		return mcp.NewToolResultError("mode is required: specify 'all' to reset all tasks or 'failed' to reset only failed tasks"), nil
	}

	taskSet, resetCount, err := s.tasks.ResetTaskSet(project, path, mode, deleteResults)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// End report session if requested
	var reportEnded bool
	if endReport {
		if endErr := s.projects.EndReport(project); endErr != nil {
			// Not a fatal error - report might not be active
			s.logger.Warnf("Could not end report session: %v", endErr)
		} else {
			reportEnded = true
		}
	}

	// Build response
	result := map[string]interface{}{
		"project":     project,
		"path":        path,
		"mode":        mode,
		"tasks_reset": resetCount,
		"task_set":    taskSet,
	}

	// Add message based on what happened
	if mode == "all" {
		result["message"] = fmt.Sprintf("Reset %d tasks to waiting status.", resetCount)
	} else {
		result["message"] = fmt.Sprintf("Reset %d failed tasks to waiting status.", resetCount)
	}

	// Add reminder if report was ended
	if reportEnded {
		result["report_ended"] = true
		result["reminder"] = "Call report_start with title and optional intro before running tasks to initialize a new report."
	}

	return createJSONResult(result)
}

// handleTaskCreate handles the task_create MCP tool
func (s *Server) handleTaskCreate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	title := mcp.ParseString(req, "title", "")
	taskType := mcp.ParseString(req, "type", "")
	instructionsFile := mcp.ParseString(req, "instructions_file", "")
	instructionsFileSource := mcp.ParseString(req, "instructions_file_source", "")
	instructionsText := mcp.ParseString(req, "instructions_text", "")
	prompt := mcp.ParseString(req, "prompt", "")
	llmModelID := mcp.ParseString(req, "llm_model_id", "")
	qaEnabled := mcp.ParseBoolean(req, "qa_enabled", false)
	qaInstructionsFile := mcp.ParseString(req, "qa_instructions_file", "")
	qaInstructionsFileSource := mcp.ParseString(req, "qa_instructions_file_source", "")
	qaInstructionsText := mcp.ParseString(req, "qa_instructions_text", "")
	qaPrompt := mcp.ParseString(req, "qa_prompt", "")
	qaLLMModelID := mcp.ParseString(req, "qa_llm_model_id", "")

	s.logToolCall(global.ToolTaskCreate, map[string]string{"project": project, "path": path, "title": title})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path is required"), nil
	}
	if title == "" {
		return mcp.NewToolResultError("title is required"), nil
	}

	// Validate instructions files exist before creating task
	if instructionsFile != "" {
		if err := s.validateInstructionsFile(project, instructionsFile, instructionsFileSource); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	if qaEnabled && qaInstructionsFile != "" {
		if err := s.validateInstructionsFile(project, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("QA %s", err.Error())), nil
		}
	}

	work := &global.WorkExecution{
		InstructionsFile:       instructionsFile,
		InstructionsFileSource: instructionsFileSource,
		InstructionsText:       instructionsText,
		Prompt:                 prompt,
		LLMModelID:             llmModelID,
		Status:                 global.ExecutionStatusWaiting,
	}

	var qa *global.QAExecution
	if qaEnabled {
		qa = &global.QAExecution{
			Enabled:                true,
			InstructionsFile:       qaInstructionsFile,
			InstructionsFileSource: qaInstructionsFileSource,
			InstructionsText:       qaInstructionsText,
			Prompt:                 qaPrompt,
			LLMModelID:             qaLLMModelID,
		}
	}

	task, err := s.tasks.CreateTask(project, path, title, taskType, work, qa)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(task)
}

// handleTaskGet handles the task_get MCP tool
func (s *Server) handleTaskGet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	taskUUID := mcp.ParseString(req, "uuid", "")
	taskID := int(mcp.ParseFloat64(req, "id", -1))
	path := mcp.ParseString(req, "path", "")

	s.logToolCall(global.ToolTaskGet, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	// Support getting by UUID or by path+ID
	if taskUUID != "" {
		task, taskSetPath, err := s.tasks.GetTask(project, taskUUID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Get taskset to include template info
		taskset, err := s.tasks.GetTaskSet(project, taskSetPath)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result := map[string]interface{}{
			"task":                     task,
			"path":                     taskSetPath,
			"worker_response_template": taskset.WorkerResponseTemplate,
		}
		return createJSONResult(result)
	}

	if path != "" && taskID >= 0 {
		task, err := s.tasks.GetTaskByID(project, path, taskID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		// Get taskset to include template info
		taskset, err := s.tasks.GetTaskSet(project, path)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		result := map[string]interface{}{
			"task":                     task,
			"worker_response_template": taskset.WorkerResponseTemplate,
		}
		return createJSONResult(result)
	}

	return mcp.NewToolResultError("either uuid or (path and id) is required"), nil
}

// handleTaskList handles the task_list MCP tool
func (s *Server) handleTaskList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	path := mcp.ParseString(req, "path", "")
	status := mcp.ParseString(req, "status", "")
	taskType := mcp.ParseString(req, "type", "")
	offset := int(mcp.ParseFloat64(req, "offset", 0))
	limit := int(mcp.ParseFloat64(req, "limit", float64(global.DefaultLimit)))

	s.logToolCall(global.ToolTaskList, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}

	result, err := s.tasks.ListTasks(project, path, status, taskType, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

// handleTaskUpdate handles the task_update MCP tool
func (s *Server) handleTaskUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	taskUUID := mcp.ParseString(req, "uuid", "")
	title := mcp.ParseString(req, "title", "")
	taskType := mcp.ParseString(req, "type", "")
	workStatus := mcp.ParseString(req, "work_status", "")

	// Work execution fields
	instructionsFile := mcp.ParseString(req, "instructions_file", "")
	instructionsFileSource := mcp.ParseString(req, "instructions_file_source", "")
	instructionsText := mcp.ParseString(req, "instructions_text", "")
	prompt := mcp.ParseString(req, "prompt", "")
	llmModelID := mcp.ParseString(req, "llm_model_id", "")

	// QA execution fields
	qaInstructionsFile := mcp.ParseString(req, "qa_instructions_file", "")
	qaInstructionsFileSource := mcp.ParseString(req, "qa_instructions_file_source", "")
	qaInstructionsText := mcp.ParseString(req, "qa_instructions_text", "")
	qaPrompt := mcp.ParseString(req, "qa_prompt", "")
	qaLLMModelID := mcp.ParseString(req, "qa_llm_model_id", "")

	s.logToolCall(global.ToolTaskUpdate, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if taskUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}

	// Validate instructions files if being updated
	if instructionsFile != "" {
		if err := s.validateInstructionsFile(project, instructionsFile, instructionsFileSource); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	if qaInstructionsFile != "" {
		if err := s.validateInstructionsFile(project, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("QA %s", err.Error())), nil
		}
	}

	updates := make(map[string]interface{})
	if title != "" {
		updates["title"] = title
	}
	if taskType != "" {
		updates["type"] = taskType
	}
	if workStatus != "" {
		updates["work_status"] = workStatus
	}

	// Work execution updates
	workUpdates := make(map[string]interface{})
	if instructionsFile != "" {
		workUpdates["instructions_file"] = instructionsFile
	}
	if instructionsFileSource != "" {
		workUpdates["instructions_file_source"] = instructionsFileSource
	}
	if instructionsText != "" {
		workUpdates["instructions_text"] = instructionsText
	}
	if prompt != "" {
		workUpdates["prompt"] = prompt
	}
	if llmModelID != "" {
		workUpdates["llm_model_id"] = llmModelID
	}
	if len(workUpdates) > 0 {
		updates["work"] = workUpdates
	}

	// QA execution updates
	qaUpdates := make(map[string]interface{})
	if qaInstructionsFile != "" {
		qaUpdates["instructions_file"] = qaInstructionsFile
	}
	if qaInstructionsFileSource != "" {
		qaUpdates["instructions_file_source"] = qaInstructionsFileSource
	}
	if qaInstructionsText != "" {
		qaUpdates["instructions_text"] = qaInstructionsText
	}
	if qaPrompt != "" {
		qaUpdates["prompt"] = qaPrompt
	}
	if qaLLMModelID != "" {
		qaUpdates["llm_model_id"] = qaLLMModelID
	}
	if len(qaUpdates) > 0 {
		updates["qa"] = qaUpdates
	}

	task, err := s.tasks.UpdateTask(project, taskUUID, updates)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(task)
}

// handleTaskDelete handles the task_delete MCP tool
func (s *Server) handleTaskDelete(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(req, "project", "")
	taskUUID := mcp.ParseString(req, "uuid", "")

	s.logToolCall(global.ToolTaskDelete, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return mcp.NewToolResultError("project is required"), nil
	}
	if taskUUID == "" {
		return mcp.NewToolResultError("uuid is required"), nil
	}

	if err := s.tasks.DeleteTask(project, taskUUID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"uuid":    taskUUID,
		"deleted": true,
	}

	return createJSONResult(result)
}

// validateInstructionsFile checks if an instructions file exists at the given source.
// Returns an error if the file does not exist or cannot be accessed.
// If instructionsFile is empty, returns nil (no validation needed).
func (s *Server) validateInstructionsFile(project, instructionsFile, instructionsFileSource string) error {
	if instructionsFile == "" {
		return nil
	}

	source := instructionsFileSource
	if source == "" {
		source = "project" // Default
	}

	switch source {
	case "project":
		if project == "" {
			return nil // Cannot validate without project context
		}
		_, err := s.projects.GetFile(project, instructionsFile, 0, 0)
		if err != nil {
			return fmt.Errorf("instructions file not found in project: %s", instructionsFile)
		}
		return nil

	case "playbook":
		if s.playbooks == nil {
			return fmt.Errorf("playbooks service not available")
		}
		// instructions_file should be "playbook-name/path/to/file.md"
		parts := strings.SplitN(instructionsFile, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid playbook instructions_file format (expected 'playbook-name/path'): %s", instructionsFile)
		}
		playbookName := parts[0]
		path := parts[1]

		_, err := s.playbooks.GetFile(playbookName, path, 0, 0)
		if err != nil {
			return fmt.Errorf("instructions file not found in playbook %s: %s", playbookName, path)
		}
		return nil

	case "reference":
		if s.reference == nil {
			return fmt.Errorf("reference service not available")
		}
		_, err := s.reference.Get(instructionsFile, 0, 0)
		if err != nil {
			return fmt.Errorf("instructions file not found in reference: %s", instructionsFile)
		}
		return nil

	default:
		return fmt.Errorf("invalid instructions_file_source: %s (must be project, playbook, or reference)", source)
	}
}

// loadSchemaContent loads schema content from a path.
// The path format determines the source:
// - "playbook-name/path/file.json" -> load from playbook
// - If it starts with '{' -> treat as inline JSON schema
// - Otherwise -> return empty (cannot validate without project context)
func (s *Server) loadSchemaContent(schemaPath string) string {
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
		if len(parts) == 2 && s.playbooks != nil {
			playbookName := parts[0]
			path := parts[1]
			if item, err := s.playbooks.GetFile(playbookName, path, 0, 0); err == nil {
				return item.Content
			}
		}
	}

	// Cannot load from project files at task set creation time
	// (we don't have project context yet, and project files may not exist)
	return ""
}
