/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/PivotLLM/Maestro/global"
)

// List Management Handlers

func (s *Server) handleListList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	offset := int(mcp.ParseFloat64(request, "offset", 0))
	limit := int(mcp.ParseFloat64(request, "limit", 0))

	s.logToolCall(global.ToolListList, map[string]string{"source": source, "project": project, "playbook": playbook})

	result, err := s.lists.List(source, project, playbook, offset, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

func (s *Server) handleListGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")

	s.logToolCall(global.ToolListGet, map[string]string{"source": source, "list": listName})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}

	result, err := s.lists.Get(source, project, playbook, listName)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

func (s *Server) handleListGetSummary(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	completeFilter := mcp.ParseString(request, "complete", "")
	offset := int(mcp.ParseFloat64(request, "offset", 0))
	limit := int(mcp.ParseFloat64(request, "limit", 0))

	s.logToolCall(global.ToolListGetSummary, map[string]string{"source": source, "list": listName, "complete": completeFilter})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}

	result, err := s.lists.GetSummary(source, project, playbook, listName, completeFilter, offset, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

func (s *Server) handleListCreate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	name := mcp.ParseString(request, "name", "")
	description := mcp.ParseString(request, "description", "")

	s.logToolCall(global.ToolListCreate, map[string]string{"source": source, "list": listName, "name": name})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	// Parse optional initial items
	var items []global.ListItem
	args := request.GetArguments()
	if val, ok := args["items"]; ok {
		if itemsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(itemsData, &items)
		}
	}

	if err := s.lists.Create(source, project, playbook, listName, name, description, items); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"name":    name,
		"created": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListDelete(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")

	s.logToolCall(global.ToolListDelete, map[string]string{"source": source, "list": listName})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}

	if err := s.lists.Delete(source, project, playbook, listName); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"deleted": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	newListName := mcp.ParseString(request, "new_list", "")

	s.logToolCall(global.ToolListRename, map[string]string{"source": source, "list": listName, "new_list": newListName})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if newListName == "" {
		return mcp.NewToolResultError("new_list parameter is required"), nil
	}

	if err := s.lists.Rename(source, project, playbook, listName, newListName); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"old_list": listName,
		"new_list": newListName,
		"renamed":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListCopy(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Source parameters
	fromSource := mcp.ParseString(request, "from_source", "")
	fromProject := mcp.ParseString(request, "from_project", "")
	fromPlaybook := mcp.ParseString(request, "from_playbook", "")
	fromList := mcp.ParseString(request, "from_list", "")

	// Destination parameters
	toSource := mcp.ParseString(request, "to_source", "")
	toProject := mcp.ParseString(request, "to_project", "")
	toPlaybook := mcp.ParseString(request, "to_playbook", "")
	toList := mcp.ParseString(request, "to_list", "")

	// Sampling
	sample := int(mcp.ParseFloat64(request, "sample", 0))

	// Build cleaner source/destination strings for logging
	fromStr := fromList
	if fromSource != "" {
		fromStr = fromSource + ":" + fromList
	}
	if fromProject != "" {
		fromStr = "project(" + fromProject + "):" + fromList
	} else if fromPlaybook != "" {
		fromStr = "playbook(" + fromPlaybook + "):" + fromList
	}
	toStr := toList
	if toSource != "" {
		toStr = toSource + ":" + toList
	}
	if toProject != "" {
		toStr = "project(" + toProject + "):" + toList
	} else if toPlaybook != "" {
		toStr = "playbook(" + toPlaybook + "):" + toList
	}
	if sample > 0 {
		s.logger.Infof("Tool %s called: copied %s to %s (sample=%d)", global.ToolListCopy, fromStr, toStr, sample)
	} else {
		s.logger.Infof("Tool %s called: copied %s to %s", global.ToolListCopy, fromStr, toStr)
	}

	if fromList == "" {
		return mcp.NewToolResultError("from_list parameter is required"), nil
	}
	if toList == "" {
		return mcp.NewToolResultError("to_list parameter is required"), nil
	}

	if err := s.lists.Copy(
		fromSource, fromProject, fromPlaybook, fromList,
		toSource, toProject, toPlaybook, toList,
		sample,
	); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"from_list": fromList,
		"to_list":   toList,
		"copied":    true,
	}
	if sample > 0 {
		result["sample"] = sample
	}

	return createJSONResult(result)
}

// List Item Management Handlers

func (s *Server) handleListItemAdd(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	title := mcp.ParseString(request, "title", "")
	content := mcp.ParseString(request, "content", "")
	sourceDoc := mcp.ParseString(request, "source_doc", "")
	section := mcp.ParseString(request, "section", "")

	s.logToolCall(global.ToolListItemAdd, map[string]string{"source": source, "list": listName, "title": title})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if title == "" {
		return mcp.NewToolResultError("title parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	// Parse tags
	var tags []string
	args := request.GetArguments()
	if val, ok := args["tags"]; ok {
		if tagsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(tagsData, &tags)
		}
	}

	item := &global.ListItem{
		// ID is always auto-generated
		Title:     title,
		Content:   content,
		SourceDoc: sourceDoc,
		Section:   section,
		Tags:      tags,
	}

	assignedID, err := s.lists.AddItem(source, project, playbook, listName, item)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":  listName,
		"id":    assignedID,
		"added": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListItemUpdate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	itemID := mcp.ParseString(request, "id", "")

	s.logToolCall(global.ToolListItemUpdate, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if itemID == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	// Parse optional fields - nil means don't update
	args := request.GetArguments()

	var title, content, sourceDoc, section *string
	if val, ok := args["title"]; ok {
		if str, ok := val.(string); ok {
			title = &str
		}
	}
	if val, ok := args["content"]; ok {
		if str, ok := val.(string); ok {
			content = &str
		}
	}
	if val, ok := args["source_doc"]; ok {
		if str, ok := val.(string); ok {
			sourceDoc = &str
		}
	}
	if val, ok := args["section"]; ok {
		if str, ok := val.(string); ok {
			section = &str
		}
	}

	// Parse tags
	var tags []string
	clearTags := false
	if val, ok := args["tags"]; ok {
		if tagsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(tagsData, &tags)
		}
	}
	if val, ok := args["clear_tags"]; ok {
		if b, ok := val.(bool); ok {
			clearTags = b
		}
	}

	// Parse complete field (optional boolean pointer)
	var complete *bool
	if val, ok := args["complete"]; ok {
		if b, ok := val.(bool); ok {
			complete = &b
		}
	}

	if err := s.lists.UpdateItem(source, project, playbook, listName, itemID, title, content, sourceDoc, section, tags, clearTags, complete); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"id":      itemID,
		"updated": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListItemRemove(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	itemID := mcp.ParseString(request, "id", "")

	s.logToolCall(global.ToolListItemRemove, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if itemID == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	if err := s.lists.RemoveItem(source, project, playbook, listName, itemID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"id":      itemID,
		"removed": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListItemRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	itemID := mcp.ParseString(request, "id", "")
	newItemID := mcp.ParseString(request, "new_id", "")

	s.logToolCall(global.ToolListItemRename, map[string]string{"source": source, "list": listName, "id": itemID, "new_id": newItemID})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if itemID == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}
	if newItemID == "" {
		return mcp.NewToolResultError("new_id parameter is required"), nil
	}

	if err := s.lists.RenameItem(source, project, playbook, listName, itemID, newItemID); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"old_id":  itemID,
		"new_id":  newItemID,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleListItemGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	itemID := mcp.ParseString(request, "id", "")

	s.logToolCall(global.ToolListItemGet, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if itemID == "" {
		return mcp.NewToolResultError("id parameter is required"), nil
	}

	item, err := s.lists.GetItem(source, project, playbook, listName, itemID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(item)
}

func (s *Server) handleListItemSearch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	playbook := mcp.ParseString(request, "playbook", "")
	listName := mcp.ParseString(request, "list", "")
	query := mcp.ParseString(request, "query", "")
	sourceDoc := mcp.ParseString(request, "source_doc", "")
	section := mcp.ParseString(request, "section", "")
	completeFilter := mcp.ParseString(request, "complete", "")
	offset := int(mcp.ParseFloat64(request, "offset", 0))
	limit := int(mcp.ParseFloat64(request, "limit", 0))

	s.logToolCall(global.ToolListItemSearch, map[string]string{"source": source, "list": listName, "query": query, "complete": completeFilter})

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}

	// Parse tags
	var tags []string
	args := request.GetArguments()
	if val, ok := args["tags"]; ok {
		if tagsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(tagsData, &tags)
		}
	}

	result, err := s.lists.SearchItems(source, project, playbook, listName, query, sourceDoc, section, tags, completeFilter, offset, limit)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}

// List Task Creation Handler

func (s *Server) handleListCreateTasks(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// List source parameters
	listSource := mcp.ParseString(request, "list_source", "")
	listProject := mcp.ParseString(request, "list_project", "")
	listPlaybook := mcp.ParseString(request, "list_playbook", "")
	listName := mcp.ParseString(request, "list", "")

	// Target project and path parameters
	targetProject := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")

	// Task template parameters
	titleTemplate := mcp.ParseString(request, "title_template", "")
	taskType := mcp.ParseString(request, "type", "")
	priority := int(mcp.ParseFloat64(request, "priority", 0))

	// Runner fields
	llmModelID := mcp.ParseString(request, "llm_model_id", "")
	instructionsFile := mcp.ParseString(request, "instructions_file", "")
	instructionsFileSource := mcp.ParseString(request, "instructions_file_source", "")
	instructionsText := mcp.ParseString(request, "instructions_text", "")
	prompt := mcp.ParseString(request, "prompt", "")

	// QA fields
	qaEnabled := mcp.ParseBoolean(request, "qa_enabled", false)
	qaInstructionsFile := mcp.ParseString(request, "qa_instructions_file", "")
	qaInstructionsFileSource := mcp.ParseString(request, "qa_instructions_file_source", "")
	qaInstructionsText := mcp.ParseString(request, "qa_instructions_text", "")
	qaPrompt := mcp.ParseString(request, "qa_prompt", "")
	qaLLMModelID := mcp.ParseString(request, "qa_llm_model_id", "")

	// Sampling and parallel execution
	sample := int(mcp.ParseFloat64(request, "sample", 0))
	parallel := mcp.ParseBoolean(request, "parallel", false)

	// Log with sample info if specified
	logParams := map[string]string{"list": listName, "project": targetProject, "type": taskType}
	if sample > 0 {
		logParams["sample"] = fmt.Sprintf("%d", sample)
	}
	s.logToolCall(global.ToolListCreateTasks, logParams)

	if listName == "" {
		return mcp.NewToolResultError("list parameter is required"), nil
	}
	if targetProject == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if taskType == "" {
		return mcp.NewToolResultError("type parameter is required"), nil
	}

	// Validate instructions files exist before creating tasks
	if instructionsFile != "" {
		if err := s.validateInstructionsFile(targetProject, instructionsFile, instructionsFileSource); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	if qaEnabled && qaInstructionsFile != "" {
		if err := s.validateInstructionsFile(targetProject, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("QA %s", err.Error())), nil
		}
	}

	// Build QA execution if enabled
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

	result, err := s.lists.CreateTasks(
		s.tasks,
		listSource, listProject, listPlaybook, listName,
		targetProject, path,
		titleTemplate, taskType, priority,
		llmModelID, instructionsFile, instructionsFileSource, instructionsText, prompt,
		qa,
		sample,
		parallel,
	)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(result)
}
