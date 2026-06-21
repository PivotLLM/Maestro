/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"fmt"
	"strings"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/runner"
	templatespkg "github.com/PivotLLM/Maestro/templates"
)

// handleTaskSetCreate handles the taskset_create MCP tool
func (p *Provider) handleTaskSetCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	title := parseString(call.Args, "title", "")
	description := parseString(call.Args, "description", "")
	parallel := parseBool(call.Args, "parallel", false)
	maxWorker := int(parseFloat64(call.Args, "max_worker", 0))
	maxQA := int(parseFloat64(call.Args, "max_qa", 0))
	workerResponseTemplate := parseString(call.Args, "worker_response_template", "")
	workerReportTemplate := parseString(call.Args, "worker_report_template", "")
	qaResponseTemplate := parseString(call.Args, "qa_response_template", "")
	qaReportTemplate := parseString(call.Args, "qa_report_template", "")

	p.logToolCall(global.ToolTaskSetCreate, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}
	if title == "" {
		return nil, fmt.Errorf("%s", "title is required")
	}

	// Validate limits if provided
	var limits global.Limits
	if maxWorker > 0 {
		validated, err := global.ValidateMaxWorker(maxWorker)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		limits.MaxWorker = validated
	}
	if maxQA > 0 {
		validated, err := global.ValidateMaxQA(maxQA)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
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
		schemaContent := p.loadSchemaContent(qaResponseTemplate)
		if schemaContent != "" {
			if err := templatespkg.ValidateQASchema(schemaContent); err != nil {
				return &toolspec.Result{ForLLM: fmt.Sprint("invalid qa_response_template: " + err.Error()), IsError: true}, nil
			}
		}
	}

	skipValidation := parseBool(call.Args, "skip_validation", false)
	callbackURL := parseString(call.Args, "callback_url", "")

	taskSet, err := p.tasks.CreateTaskSet(project, path, title, description, templates, parallel, limits, skipValidation, callbackURL)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetGet handles the taskset_get MCP tool
func (p *Provider) handleTaskSetGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolTaskSetGet, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}

	taskSet, err := p.tasks.GetTaskSet(project, path)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetList handles the taskset_list MCP tool
func (p *Provider) handleTaskSetList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	pathPrefix := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolTaskSetList, map[string]string{"project": project, "path": pathPrefix})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	result, err := p.tasks.ListTaskSets(project, pathPrefix)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleTaskSetUpdate handles the taskset_update MCP tool
func (p *Provider) handleTaskSetUpdate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	titleStr := parseString(call.Args, "title", "")
	descriptionStr := parseString(call.Args, "description", "")
	parallelStr := parseString(call.Args, "parallel", "")
	maxWorkerVal := int(parseFloat64(call.Args, "max_worker", -1))
	maxQAVal := int(parseFloat64(call.Args, "max_qa", -1))
	workerResponseTemplate := parseString(call.Args, "worker_response_template", "")
	workerReportTemplate := parseString(call.Args, "worker_report_template", "")
	qaResponseTemplate := parseString(call.Args, "qa_response_template", "")
	qaReportTemplate := parseString(call.Args, "qa_report_template", "")

	p.logToolCall(global.ToolTaskSetUpdate, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
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
				return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
			}
			limits.MaxWorker = validated
		}
		if maxQAVal >= 0 {
			validated, err := global.ValidateMaxQA(maxQAVal)
			if err != nil {
				return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
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
		schemaContent := p.loadSchemaContent(qaResponseTemplate)
		if schemaContent != "" {
			if err := templatespkg.ValidateQASchema(schemaContent); err != nil {
				return &toolspec.Result{ForLLM: fmt.Sprint("invalid qa_response_template: " + err.Error()), IsError: true}, nil
			}
		}
	}

	// Handle skip_validation update
	var skipValidation *bool
	skipValidationStr := parseString(call.Args, "skip_validation", "")
	if skipValidationStr != "" {
		skipValidationVal := skipValidationStr == "true"
		skipValidation = &skipValidationVal
	}

	// Handle callback_url update
	var callbackURL *string
	callbackURLStr := parseString(call.Args, "callback_url", "")
	if callbackURLStr != "" {
		callbackURL = &callbackURLStr
	}

	taskSet, err := p.tasks.UpdateTaskSet(project, path, title, description, templates, parallel, limits, skipValidation, callbackURL)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(taskSet)
}

// handleTaskSetDelete handles the taskset_delete MCP tool
func (p *Provider) handleTaskSetDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolTaskSetDelete, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}

	if err := p.tasks.DeleteTaskSet(project, path); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"deleted": true,
	}

	return createJSONResult(result)
}

// handleTaskSetReset handles the taskset_reset MCP tool
func (p *Provider) handleTaskSetReset(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	mode := parseString(call.Args, "mode", "")
	deleteResults := parseBool(call.Args, "delete_results", true)
	endReport := parseBool(call.Args, "end_report", false)

	p.logToolCall(global.ToolTaskSetReset, map[string]string{"project": project, "path": path, "mode": mode})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}
	if mode == "" {
		return nil, fmt.Errorf("%s", "mode is required: specify 'all' to reset all tasks or 'failed' to reset only failed tasks")
	}

	taskSet, resetCount, err := p.tasks.ResetTaskSet(project, path, mode, deleteResults)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	// End report session if requested
	var reportEnded bool
	if endReport {
		if endErr := p.projects.EndReport(project); endErr != nil {
			// Not a fatal error - report might not be active
			p.logger.Warnf("Could not end report session: %v", endErr)
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
func (p *Provider) handleTaskCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	title := parseString(call.Args, "title", "")
	taskType := parseString(call.Args, "type", "")
	instructionsFile := parseString(call.Args, "instructions_file", "")
	instructionsFileSource := parseString(call.Args, "instructions_file_source", "")
	instructionsText := parseString(call.Args, "instructions_text", "")
	prompt := parseString(call.Args, "prompt", "")
	llmModelID := parseString(call.Args, "llm_model_id", "")
	qaEnabled := parseBool(call.Args, "qa_enabled", false)
	qaInstructionsFile := parseString(call.Args, "qa_instructions_file", "")
	qaInstructionsFileSource := parseString(call.Args, "qa_instructions_file_source", "")
	qaInstructionsText := parseString(call.Args, "qa_instructions_text", "")
	qaPrompt := parseString(call.Args, "qa_prompt", "")
	qaLLMModelID := parseString(call.Args, "qa_llm_model_id", "")

	p.logToolCall(global.ToolTaskCreate, map[string]string{"project": project, "path": path, "title": title})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}
	if title == "" {
		return nil, fmt.Errorf("%s", "title is required")
	}

	// Validate instructions files exist before creating task
	if instructionsFile != "" {
		if err := p.validateInstructionsFile(project, instructionsFile, instructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
	}
	if qaEnabled && qaInstructionsFile != "" {
		if err := p.validateInstructionsFile(project, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("QA %s", err.Error())), IsError: true}, nil
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

	task, err := p.tasks.CreateTask(project, path, title, taskType, work, qa)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(task)
}

// handleTaskGet handles the task_get MCP tool
func (p *Provider) handleTaskGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	taskUUID := parseString(call.Args, "uuid", "")
	taskID := int(parseFloat64(call.Args, "id", -1))
	path := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolTaskGet, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	// Support getting by UUID or by path+ID
	if taskUUID != "" {
		task, taskSetPath, err := p.tasks.GetTask(project, taskUUID)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		// Get taskset to include template info
		taskset, err := p.tasks.GetTaskSet(project, taskSetPath)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result := map[string]interface{}{
			"task":                     task,
			"path":                     taskSetPath,
			"worker_response_template": taskset.WorkerResponseTemplate,
		}
		return createJSONResult(result)
	}

	if path != "" && taskID >= 0 {
		task, err := p.tasks.GetTaskByID(project, path, taskID)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		// Get taskset to include template info
		taskset, err := p.tasks.GetTaskSet(project, path)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result := map[string]interface{}{
			"task":                     task,
			"worker_response_template": taskset.WorkerResponseTemplate,
		}
		return createJSONResult(result)
	}

	return &toolspec.Result{ForLLM: fmt.Sprint("either uuid or (path and id) is required"), IsError: true}, nil
}

// handleTaskList handles the task_list MCP tool
func (p *Provider) handleTaskList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	status := parseString(call.Args, "status", "")
	taskType := parseString(call.Args, "type", "")
	offset := int(parseFloat64(call.Args, "offset", 0))
	limit := int(parseFloat64(call.Args, "limit", float64(global.DefaultLimit)))

	p.logToolCall(global.ToolTaskList, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	result, err := p.tasks.ListTasks(project, path, status, taskType, limit, offset)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleTaskUpdate handles the task_update MCP tool
func (p *Provider) handleTaskUpdate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	taskUUID := parseString(call.Args, "uuid", "")
	title := parseString(call.Args, "title", "")
	taskType := parseString(call.Args, "type", "")
	workStatus := parseString(call.Args, "work_status", "")

	// Work execution fields
	instructionsFile := parseString(call.Args, "instructions_file", "")
	instructionsFileSource := parseString(call.Args, "instructions_file_source", "")
	instructionsText := parseString(call.Args, "instructions_text", "")
	prompt := parseString(call.Args, "prompt", "")
	llmModelID := parseString(call.Args, "llm_model_id", "")

	// QA execution fields
	qaInstructionsFile := parseString(call.Args, "qa_instructions_file", "")
	qaInstructionsFileSource := parseString(call.Args, "qa_instructions_file_source", "")
	qaInstructionsText := parseString(call.Args, "qa_instructions_text", "")
	qaPrompt := parseString(call.Args, "qa_prompt", "")
	qaLLMModelID := parseString(call.Args, "qa_llm_model_id", "")

	p.logToolCall(global.ToolTaskUpdate, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if taskUUID == "" {
		return nil, fmt.Errorf("%s", "uuid is required")
	}

	// Validate instructions files if being updated
	if instructionsFile != "" {
		if err := p.validateInstructionsFile(project, instructionsFile, instructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
	}
	if qaInstructionsFile != "" {
		if err := p.validateInstructionsFile(project, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("QA %s", err.Error())), IsError: true}, nil
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

	task, err := p.tasks.UpdateTask(project, taskUUID, updates)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(task)
}

// handleTaskDelete handles the task_delete MCP tool
func (p *Provider) handleTaskDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	taskUUID := parseString(call.Args, "uuid", "")

	p.logToolCall(global.ToolTaskDelete, map[string]string{"project": project, "uuid": taskUUID})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}
	if taskUUID == "" {
		return nil, fmt.Errorf("%s", "uuid is required")
	}

	if err := p.tasks.DeleteTask(project, taskUUID); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
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
func (p *Provider) validateInstructionsFile(project, instructionsFile, instructionsFileSource string) error {
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
		_, err := p.projects.GetFile(project, instructionsFile, 0, 0)
		if err != nil {
			return fmt.Errorf("instructions file not found in project: %s", instructionsFile)
		}
		return nil

	case "playbook":
		if p.playbooks == nil {
			return fmt.Errorf("playbooks service not available")
		}
		// instructions_file should be "playbook-name/path/to/file.md"
		parts := strings.SplitN(instructionsFile, "/", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid playbook instructions_file format (expected 'playbook-name/path'): %s", instructionsFile)
		}
		playbookName := parts[0]
		path := parts[1]

		_, err := p.playbooks.GetFile(playbookName, path, 0, 0)
		if err != nil {
			return fmt.Errorf("instructions file not found in playbook %s: %s", playbookName, path)
		}
		return nil

	case "reference":
		if p.reference == nil {
			return fmt.Errorf("reference service not available")
		}
		_, err := p.reference.Get(instructionsFile, 0, 0)
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
func (p *Provider) loadSchemaContent(schemaPath string) string {
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
		if len(parts) == 2 && p.playbooks != nil {
			playbookName := parts[0]
			path := parts[1]
			if item, err := p.playbooks.GetFile(playbookName, path, 0, 0); err == nil {
				return item.Content
			}
		}
	}

	// Cannot load from project files at task set creation time
	// (we don't have project context yet, and project files may not exist)
	return ""
}

// handleTaskDispatch handles the task_dispatch MCP tool.
// Creates a single-task taskset and runs it asynchronously, returning immediately.
func (p *Provider) handleTaskDispatch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	title := parseString(call.Args, "title", "")
	llmModelID := parseString(call.Args, "llm_model_id", "")
	prompt := parseString(call.Args, "prompt", "")
	instructionsText := parseString(call.Args, "instructions_text", "")
	instructionsFile := parseString(call.Args, "instructions_file", "")
	instructionsFileSource := parseString(call.Args, "instructions_file_source", "")
	callbackURL := parseString(call.Args, "callback_url", "")

	p.logToolCall(global.ToolTaskDispatch, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project is required")
	}

	// At least one prompt source is required
	if prompt == "" && instructionsText == "" && instructionsFile == "" {
		return nil, fmt.Errorf("%s", "at least one of prompt, instructions_text, or instructions_file is required")
	}

	dispatchReq := &runner.DispatchRequest{
		Project:                project,
		Path:                   path,
		Title:                  title,
		LLMModelID:             llmModelID,
		Prompt:                 prompt,
		InstructionsText:       instructionsText,
		InstructionsFile:       instructionsFile,
		InstructionsFileSource: instructionsFileSource,
		CallbackURL:            callbackURL,
	}

	result, err := p.runner.RunDispatch(dispatchReq, completionSink(call))
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}
