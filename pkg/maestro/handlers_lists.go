/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"encoding/json"
	"fmt"

	"github.com/PivotLLM/Maestro/global"
)

// List Management Handlers

func (p *Provider) handleListList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	offset := int(parseFloat64(call.Args, "offset", 0))
	limit := int(parseFloat64(call.Args, "limit", 0))

	p.logToolCall(global.ToolListList, map[string]string{"source": source, "project": project, "playbook": playbook})

	result, err := p.lists.List(source, project, playbook, offset, limit)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

func (p *Provider) handleListGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")

	p.logToolCall(global.ToolListGet, map[string]string{"source": source, "list": listName})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}

	result, err := p.lists.Get(source, project, playbook, listName)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

func (p *Provider) handleListGetSummary(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	completeFilter := parseString(call.Args, "complete", "")
	offset := int(parseFloat64(call.Args, "offset", 0))
	limit := int(parseFloat64(call.Args, "limit", 0))

	p.logToolCall(global.ToolListGetSummary, map[string]string{"source": source, "list": listName, "complete": completeFilter})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}

	result, err := p.lists.GetSummary(source, project, playbook, listName, completeFilter, offset, limit)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

func (p *Provider) handleListCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	name := parseString(call.Args, "name", "")
	description := parseString(call.Args, "description", "")

	p.logToolCall(global.ToolListCreate, map[string]string{"source": source, "list": listName, "name": name})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	// Parse optional initial items
	var items []global.ListItem
	args := call.Args
	if val, ok := args["items"]; ok {
		if itemsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(itemsData, &items)
		}
	}

	if err := p.lists.Create(source, project, playbook, listName, name, description, items); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"name":    name,
		"created": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")

	p.logToolCall(global.ToolListDelete, map[string]string{"source": source, "list": listName})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}

	if err := p.lists.Delete(source, project, playbook, listName); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"deleted": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	newListName := parseString(call.Args, "new_list", "")

	p.logToolCall(global.ToolListRename, map[string]string{"source": source, "list": listName, "new_list": newListName})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if newListName == "" {
		return nil, fmt.Errorf("%s", "new_list parameter is required")
	}

	if err := p.lists.Rename(source, project, playbook, listName, newListName); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"old_list": listName,
		"new_list": newListName,
		"renamed":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListCopy(call *toolspec.ToolCall) (*toolspec.Result, error) {
	// Source parameters
	fromSource := parseString(call.Args, "from_source", "")
	fromProject := parseString(call.Args, "from_project", "")
	fromPlaybook := parseString(call.Args, "from_playbook", "")
	fromList := parseString(call.Args, "from_list", "")

	// Destination parameters
	toSource := parseString(call.Args, "to_source", "")
	toProject := parseString(call.Args, "to_project", "")
	toPlaybook := parseString(call.Args, "to_playbook", "")
	toList := parseString(call.Args, "to_list", "")

	// Sampling
	sample := int(parseFloat64(call.Args, "sample", 0))

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
		p.logger.Infof("Tool %s called: copied %s to %s (sample=%d)", global.ToolListCopy, fromStr, toStr, sample)
	} else {
		p.logger.Infof("Tool %s called: copied %s to %s", global.ToolListCopy, fromStr, toStr)
	}

	if fromList == "" {
		return nil, fmt.Errorf("%s", "from_list parameter is required")
	}
	if toList == "" {
		return nil, fmt.Errorf("%s", "to_list parameter is required")
	}

	if err := p.lists.Copy(
		fromSource, fromProject, fromPlaybook, fromList,
		toSource, toProject, toPlaybook, toList,
		sample,
	); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
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

func (p *Provider) handleListItemAdd(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	title := parseString(call.Args, "title", "")
	content := parseString(call.Args, "content", "")
	sourceDoc := parseString(call.Args, "source_doc", "")
	section := parseString(call.Args, "section", "")

	p.logToolCall(global.ToolListItemAdd, map[string]string{"source": source, "list": listName, "title": title})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if title == "" {
		return nil, fmt.Errorf("%s", "title parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	// Parse tags
	var tags []string
	args := call.Args
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

	assignedID, err := p.lists.AddItem(source, project, playbook, listName, item)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":  listName,
		"id":    assignedID,
		"added": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListItemUpdate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	itemID := parseString(call.Args, "id", "")

	p.logToolCall(global.ToolListItemUpdate, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if itemID == "" {
		return nil, fmt.Errorf("%s", "id parameter is required")
	}

	// Parse optional fields - nil means don't update
	args := call.Args

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

	if err := p.lists.UpdateItem(source, project, playbook, listName, itemID, title, content, sourceDoc, section, tags, clearTags, complete); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"id":      itemID,
		"updated": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListItemRemove(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	itemID := parseString(call.Args, "id", "")

	p.logToolCall(global.ToolListItemRemove, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if itemID == "" {
		return nil, fmt.Errorf("%s", "id parameter is required")
	}

	if err := p.lists.RemoveItem(source, project, playbook, listName, itemID); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"id":      itemID,
		"removed": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListItemRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	itemID := parseString(call.Args, "id", "")
	newItemID := parseString(call.Args, "new_id", "")

	p.logToolCall(global.ToolListItemRename, map[string]string{"source": source, "list": listName, "id": itemID, "new_id": newItemID})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if itemID == "" {
		return nil, fmt.Errorf("%s", "id parameter is required")
	}
	if newItemID == "" {
		return nil, fmt.Errorf("%s", "new_id parameter is required")
	}

	if err := p.lists.RenameItem(source, project, playbook, listName, itemID, newItemID); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"list":    listName,
		"old_id":  itemID,
		"new_id":  newItemID,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handleListItemGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	itemID := parseString(call.Args, "id", "")

	p.logToolCall(global.ToolListItemGet, map[string]string{"source": source, "list": listName, "id": itemID})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if itemID == "" {
		return nil, fmt.Errorf("%s", "id parameter is required")
	}

	item, err := p.lists.GetItem(source, project, playbook, listName, itemID)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}

func (p *Provider) handleListItemSearch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")
	listName := parseString(call.Args, "list", "")
	query := parseString(call.Args, "query", "")
	sourceDoc := parseString(call.Args, "source_doc", "")
	section := parseString(call.Args, "section", "")
	completeFilter := parseString(call.Args, "complete", "")
	offset := int(parseFloat64(call.Args, "offset", 0))
	limit := int(parseFloat64(call.Args, "limit", 0))

	p.logToolCall(global.ToolListItemSearch, map[string]string{"source": source, "list": listName, "query": query, "complete": completeFilter})

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}

	// Parse tags
	var tags []string
	args := call.Args
	if val, ok := args["tags"]; ok {
		if tagsData, err := json.Marshal(val); err == nil {
			_ = json.Unmarshal(tagsData, &tags)
		}
	}

	result, err := p.lists.SearchItems(source, project, playbook, listName, query, sourceDoc, section, tags, completeFilter, offset, limit)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}

// List Task Creation Handler

func (p *Provider) handleListCreateTasks(call *toolspec.ToolCall) (*toolspec.Result, error) {
	// List source parameters
	listSource := parseString(call.Args, "list_source", "")
	listProject := parseString(call.Args, "list_project", "")
	listPlaybook := parseString(call.Args, "list_playbook", "")
	listName := parseString(call.Args, "list", "")

	// Target project and path parameters
	targetProject := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")

	// Task template parameters
	titleTemplate := parseString(call.Args, "title_template", "")
	taskType := parseString(call.Args, "type", "")
	priority := int(parseFloat64(call.Args, "priority", 0))

	// Runner fields
	llmModelID := parseString(call.Args, "llm_model_id", "")
	instructionsFile := parseString(call.Args, "instructions_file", "")
	instructionsFileSource := parseString(call.Args, "instructions_file_source", "")
	instructionsText := parseString(call.Args, "instructions_text", "")
	prompt := parseString(call.Args, "prompt", "")

	// QA fields
	qaEnabled := parseBool(call.Args, "qa_enabled", false)
	qaInstructionsFile := parseString(call.Args, "qa_instructions_file", "")
	qaInstructionsFileSource := parseString(call.Args, "qa_instructions_file_source", "")
	qaInstructionsText := parseString(call.Args, "qa_instructions_text", "")
	qaPrompt := parseString(call.Args, "qa_prompt", "")
	qaLLMModelID := parseString(call.Args, "qa_llm_model_id", "")

	// Sampling and parallel execution
	sample := int(parseFloat64(call.Args, "sample", 0))
	parallel := parseBool(call.Args, "parallel", false)

	// Log with sample info if specified
	logParams := map[string]string{"list": listName, "project": targetProject, "type": taskType}
	if sample > 0 {
		logParams["sample"] = fmt.Sprintf("%d", sample)
	}
	p.logToolCall(global.ToolListCreateTasks, logParams)

	if listName == "" {
		return nil, fmt.Errorf("%s", "list parameter is required")
	}
	if targetProject == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if taskType == "" {
		return nil, fmt.Errorf("%s", "type parameter is required")
	}

	// Validate instructions files exist before creating tasks
	if instructionsFile != "" {
		if err := p.validateInstructionsFile(targetProject, instructionsFile, instructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
	}
	if qaEnabled && qaInstructionsFile != "" {
		if err := p.validateInstructionsFile(targetProject, qaInstructionsFile, qaInstructionsFileSource); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("QA %s", err.Error())), IsError: true}, nil
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

	result, err := p.lists.CreateTasks(
		p.tasks,
		listSource, listProject, listPlaybook, listName,
		targetProject, path,
		titleTemplate, taskType, priority,
		llmModelID, instructionsFile, instructionsFileSource, instructionsText, prompt,
		qa,
		sample,
		parallel,
	)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(result)
}
