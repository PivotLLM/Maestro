/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
)

// Helper function to create JSON tool results safely
func createJSONResult(data interface{}) (*mcp.CallToolResult, error) {
	result, err := mcp.NewToolResultJSON(data)
	if err != nil {
		return mcp.NewToolResultError("Failed to create JSON result"), nil
	}
	return result, nil
}

// logToolCall logs an MCP tool invocation at INFO level
func (s *Server) logToolCall(toolName string, params map[string]string) {
	if len(params) == 0 {
		s.logger.Infof("Tool %s called", toolName)
		return
	}
	// Build params string
	var parts []string
	for k, v := range params {
		if v != "" {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(parts) == 0 {
		s.logger.Infof("Tool %s called", toolName)
	} else {
		s.logger.Infof("Tool %s called: %s", toolName, joinStrings(parts, ", "))
	}
}

// joinStrings joins string slice with separator (avoiding strings import)
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// Project tool handlers

func (s *Server) handleProjectCreate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")
	title := mcp.ParseString(request, "title", "")
	description := mcp.ParseString(request, "description", "")
	projectContext := mcp.ParseString(request, "context", "")
	status := mcp.ParseString(request, "status", "")
	disclaimerTemplate := mcp.ParseString(request, "disclaimer_template", "")

	s.logToolCall(global.ToolProjectCreate, map[string]string{"name": name})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}
	if title == "" {
		return mcp.NewToolResultError("title parameter is required"), nil
	}
	if disclaimerTemplate == "" {
		return mcp.NewToolResultError("disclaimer_template parameter is required: provide a playbook path (e.g., 'playbook-name/templates/disclaimer.md') or 'none'"), nil
	}

	proj, err := s.projects.Create(name, title, description, projectContext, status, disclaimerTemplate)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(proj)
}

func (s *Server) handleProjectGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")

	s.logToolCall(global.ToolProjectGet, map[string]string{"name": name})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	proj, err := s.projects.Get(name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(proj)
}

func (s *Server) handleProjectUpdate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")
	titleStr := mcp.ParseString(request, "title", "")
	descriptionStr := mcp.ParseString(request, "description", "")
	contextStr := mcp.ParseString(request, "context", "")
	statusStr := mcp.ParseString(request, "status", "")
	disclaimerTemplateStr := mcp.ParseString(request, "disclaimer_template", "")

	s.logToolCall(global.ToolProjectUpdate, map[string]string{"name": name, "status": statusStr})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
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

	proj, err := s.projects.Update(name, title, description, projectContext, status, disclaimerTemplate)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(proj)
}

func (s *Server) handleProjectList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	status := mcp.ParseString(request, "status", "")
	limit := int(mcp.ParseFloat64(request, "limit", 0))
	offset := int(mcp.ParseFloat64(request, "offset", 0))

	s.logToolCall(global.ToolProjectList, map[string]string{"status": status})

	result, err := s.projects.List(status, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectDelete(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")

	s.logToolCall(global.ToolProjectDelete, map[string]string{"name": name})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	if err := s.projects.Delete(name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": name,
		"deleted": true,
	}

	return createJSONResult(result)
}

// Project Log tool handlers

func (s *Server) handleProjectLogAppend(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	task := mcp.ParseString(request, "task", "")
	message := mcp.ParseString(request, "message", "")

	s.logToolCall(global.ToolProjectLogAppend, map[string]string{"project": project, "task": task})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if message == "" {
		return mcp.NewToolResultError("message parameter is required"), nil
	}

	// Append to project or task log
	if err := s.projects.AppendLog(project, task, message); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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

func (s *Server) handleProjectLogGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	task := mcp.ParseString(request, "task", "")
	limit := int(mcp.ParseFloat64(request, "limit", float64(global.DefaultLogLimit)))
	offset := int(mcp.ParseFloat64(request, "offset", 0))

	s.logToolCall(global.ToolProjectLogGet, map[string]string{"project": project, "task": task})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	logResult, err := s.projects.GetLog(project, task, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(logResult)
}

// LLM handlers

func (s *Server) handleLLMList(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logToolCall(global.ToolLLMList, nil)
	result := s.llm.ListLLMs()
	return createJSONResult(result)
}

func (s *Server) handleLLMDispatch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	llmID := mcp.ParseString(request, "llm_id", "")
	prompt := mcp.ParseString(request, "prompt", "")
	timeout := int(mcp.ParseFloat64(request, "timeout", 0))

	s.logToolCall(global.ToolLLMDispatch, map[string]string{"llm_id": llmID})

	if llmID == "" {
		return mcp.NewToolResultError("llm_id parameter is required"), nil
	}
	if prompt == "" {
		return mcp.NewToolResultError("prompt parameter is required"), nil
	}

	// Validate timeout
	if timeout > 0 {
		_, err := global.ValidateTimeout(timeout)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	// Parse context_keys and options from raw arguments if available
	var contextKeys []string
	var options *llm.DispatchOptions

	// Add timeout to options if provided
	if timeout > 0 {
		if options == nil {
			options = &llm.DispatchOptions{}
		}
		options.Timeout = timeout
	}

	req := &llm.DispatchRequest{
		LLMID:       llmID,
		Prompt:      prompt,
		ContextKeys: contextKeys,
		Options:     options,
	}

	result, err := s.llm.Dispatch(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

func (s *Server) handleLLMTest(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	llmID := mcp.ParseString(request, "llm_id", "")

	s.logToolCall(global.ToolLLMTest, map[string]string{"llm_id": llmID})

	if llmID == "" {
		return mcp.NewToolResultError("llm_id parameter is required"), nil
	}

	available, err := s.llm.TestLLM(llmID)
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

func (s *Server) handleHealth(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logToolCall(global.ToolHealth, nil)
	var issues []string

	// Check if base directory exists
	baseDir := s.config.BaseDir()
	if !dirExists(baseDir) {
		issues = append(issues, fmt.Sprintf("base directory does not exist: %s", baseDir))
	}

	// Check if at least one LLM is enabled
	if !s.config.HasEnabledLLM() {
		issues = append(issues, "no LLMs are enabled - edit config.json and set enabled: true for at least one LLM")
	}

	// Check if first run
	if s.config.IsFirstRun() {
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
		"config_path":  s.config.ConfigPath(),
		"first_run":    s.config.IsFirstRun(),
		"enabled_llms": len(s.config.EnabledLLMs()),
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
