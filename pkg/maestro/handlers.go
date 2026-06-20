/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"fmt"
	"os"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
)

// Project tool handlers

func (p *Provider) handleProjectCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")
	title := parseString(call.Args, "title", "")
	description := parseString(call.Args, "description", "")
	projectContext := parseString(call.Args, "context", "")
	status := parseString(call.Args, "status", "")
	disclaimerTemplate := parseString(call.Args, "disclaimer_template", "")

	p.logToolCall(global.ToolProjectCreate, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}
	if title == "" {
		return nil, fmt.Errorf("%s", "title parameter is required")
	}
	if disclaimerTemplate == "" {
		return &toolspec.Result{ForLLM: fmt.Sprint("disclaimer_template parameter is required: provide a playbook path (e.g., 'playbook-name/templates/disclaimer.md') or 'none'"), IsError: true}, nil
	}

	proj, err := p.projects.Create(name, title, description, projectContext, status, disclaimerTemplate)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(proj)
}

func (p *Provider) handleProjectGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolProjectGet, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	proj, err := p.projects.Get(name)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(proj)
}

func (p *Provider) handleProjectUpdate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")
	titleStr := parseString(call.Args, "title", "")
	descriptionStr := parseString(call.Args, "description", "")
	contextStr := parseString(call.Args, "context", "")
	statusStr := parseString(call.Args, "status", "")
	disclaimerTemplateStr := parseString(call.Args, "disclaimer_template", "")

	p.logToolCall(global.ToolProjectUpdate, map[string]string{"name": name, "status": statusStr})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	// Convert empty strings to nil pointers for optional fields
	var title, description, projectContext, status, disclaimerTemplate *string
	if titleStr != "" {
		title = &titleStr
	}
	if descriptionStr != "" {
		description = &descriptionStr
	}
	if contextStr != "" {
		projectContext = &contextStr
	}
	if statusStr != "" {
		status = &statusStr
	}
	if disclaimerTemplateStr != "" {
		disclaimerTemplate = &disclaimerTemplateStr
	}

	proj, err := p.projects.Update(name, title, description, projectContext, status, disclaimerTemplate)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(proj)
}

func (p *Provider) handleProjectList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	status := parseString(call.Args, "status", "")
	limit := int(parseFloat64(call.Args, "limit", 0))
	offset := int(parseFloat64(call.Args, "offset", 0))

	p.logToolCall(global.ToolProjectList, map[string]string{"status": status})

	result, err := p.projects.List(status, limit, offset)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

func (p *Provider) handleProjectDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolProjectDelete, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	if err := p.projects.Delete(name); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project": name,
		"deleted": true,
	}

	return createJSONResult(result)
}

// Project Log tool handlers

func (p *Provider) handleProjectLogAppend(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	task := parseString(call.Args, "task", "")
	message := parseString(call.Args, "message", "")

	p.logToolCall(global.ToolProjectLogAppend, map[string]string{"project": project, "task": task})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if message == "" {
		return nil, fmt.Errorf("%s", "message parameter is required")
	}

	// Append to project or task log
	if err := p.projects.AppendLog(project, task, message); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"project": project,
		"logged":  true,
	}
	if task != "" {
		result["task"] = task
	}

	return createJSONResult(result)
}

func (p *Provider) handleProjectLogGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	task := parseString(call.Args, "task", "")
	limit := int(parseFloat64(call.Args, "limit", float64(global.DefaultLogLimit)))
	offset := int(parseFloat64(call.Args, "offset", 0))

	p.logToolCall(global.ToolProjectLogGet, map[string]string{"project": project, "task": task})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

	logResult, err := p.projects.GetLog(project, task, limit, offset)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(logResult)
}

// LLM handlers

func (p *Provider) handleLLMList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolLLMList, nil)
	result := p.llm.ListLLMs()
	return createJSONResult(result)
}

func (p *Provider) handleLLMDispatch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	llmID := parseString(call.Args, "llm_id", "")
	prompt := parseString(call.Args, "prompt", "")

	p.logToolCall(global.ToolLLMDispatch, map[string]string{"llm_id": llmID})

	if llmID == "" {
		return nil, fmt.Errorf("%s", "llm_id parameter is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("%s", "prompt parameter is required")
	}

	// Parse context_keys from raw arguments if available
	var contextKeys []string

	req := &llm.DispatchRequest{
		LLMID:       llmID,
		Prompt:      prompt,
		ContextKeys: contextKeys,
	}

	result, err := p.llm.Dispatch(req)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

func (p *Provider) handleLLMTest(call *toolspec.ToolCall) (*toolspec.Result, error) {
	llmID := parseString(call.Args, "llm_id", "")

	p.logToolCall(global.ToolLLMTest, map[string]string{"llm_id": llmID})

	if llmID == "" {
		return nil, fmt.Errorf("%s", "llm_id parameter is required")
	}

	available, err := p.llm.TestLLM(llmID)
	if err != nil {
		return createJSONResult(map[string]interface{}{
			"available": false,
			"error":     err.Error(),
		})
	}

	return createJSONResult(map[string]interface{}{
		"available": available,
	})
}

// System handlers

func (p *Provider) handleHealth(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolHealth, nil)
	var issues []string

	// Check if base directory exists
	baseDir := p.config.BaseDir()
	if !dirExists(baseDir) {
		issues = append(issues, fmt.Sprintf("base directory does not exist: %s", baseDir))
	}

	// Check if at least one LLM is enabled
	if !p.config.HasEnabledLLM() {
		issues = append(issues, "no LLMs are enabled - edit config.json and set enabled: true for at least one LLM")
	}

	// Check if first run
	if p.config.IsFirstRun() {
		issues = append(issues, "this is a first run - configuration was just created, please review and configure")
	}

	// Build result
	healthy := len(issues) == 0
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}

	result := map[string]interface{}{
		"status":       status,
		"healthy":      healthy,
		"program_name": global.ProgramName,
		"version":      global.Version,
		"base_dir":     baseDir,
		"config_path":  p.config.ConfigPath(),
		"first_run":    p.config.IsFirstRun(),
		"enabled_llms": len(p.config.EnabledLLMs()),
	}

	if len(issues) > 0 {
		result["issues"] = issues
	}

	return createJSONResult(result)
}

// Helper to check if directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
