/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"fmt"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/global"
)

// Playbook tool handlers

func (p *Provider) handlePlaybookList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	p.logToolCall(global.ToolPlaybookList, nil)
	playbooks, err := p.playbooks.List()
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbooks": playbooks,
		"count":     len(playbooks),
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookCreate(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolPlaybookCreate, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	if err := p.playbooks.Create(name); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"created":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")
	newName := parseString(call.Args, "new_name", "")

	p.logToolCall(global.ToolPlaybookRename, map[string]string{"name": name, "new_name": newName})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}
	if newName == "" {
		return nil, fmt.Errorf("%s", "new_name parameter is required")
	}

	if err := p.playbooks.Rename(name, newName); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"from":    name,
		"to":      newName,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")

	p.logToolCall(global.ToolPlaybookDelete, map[string]string{"name": name})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}

	if err := p.playbooks.Delete(name); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"deleted":  true,
	}

	return createJSONResult(result)
}

// Playbook file handlers

func (p *Provider) handlePlaybookFileList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	prefix := parseString(call.Args, "prefix", "")

	p.logToolCall(global.ToolPlaybookFileList, map[string]string{"playbook": playbook})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}

	items, err := p.playbooks.ListFiles(playbook, prefix)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"files":    items,
		"count":    len(items),
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookFileGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	path := parseString(call.Args, "path", "")
	byteOffset := int64(parseFloat64(call.Args, "byte_offset", 0))
	maxBytes := int64(parseFloat64(call.Args, "max_bytes", 0))

	p.logToolCall(global.ToolPlaybookFileGet, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}

	item, err := p.playbooks.GetFile(playbook, path, byteOffset, maxBytes)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	return createJSONResult(item)
}

func (p *Provider) handlePlaybookFilePut(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	path := parseString(call.Args, "path", "")
	content := parseString(call.Args, "content", "")
	summary := parseString(call.Args, "summary", "")

	p.logToolCall(global.ToolPlaybookFilePut, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	created, err := p.playbooks.PutFile(playbook, path, content, summary)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"created":  created,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookFileAppend(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	path := parseString(call.Args, "path", "")
	content := parseString(call.Args, "content", "")
	summary := parseString(call.Args, "summary", "")

	p.logToolCall(global.ToolPlaybookFileAppend, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	err := p.playbooks.AppendFile(playbook, path, content, summary)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"success":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookFileEdit(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	path := parseString(call.Args, "path", "")
	oldString := parseString(call.Args, "old_string", "")
	newString := parseString(call.Args, "new_string", "")
	replaceAll := parseBool(call.Args, "replace_all", false)

	p.logToolCall(global.ToolPlaybookFileEdit, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if oldString == "" {
		return nil, fmt.Errorf("%s", "old_string parameter is required")
	}
	// new_string can be empty to delete the old_string

	err := p.playbooks.EditFile(playbook, path, oldString, newString, replaceAll)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"success":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookFileRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	fromPath := parseString(call.Args, "from_path", "")
	toPath := parseString(call.Args, "to_path", "")

	p.logToolCall(global.ToolPlaybookFileRename, map[string]string{"playbook": playbook, "from": fromPath, "to": toPath})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if fromPath == "" {
		return nil, fmt.Errorf("%s", "from_path parameter is required")
	}
	if toPath == "" {
		return nil, fmt.Errorf("%s", "to_path parameter is required")
	}

	if err := p.playbooks.RenameFile(playbook, fromPath, toPath); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"from":     fromPath,
		"to":       toPath,
		"renamed":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookFileDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	path := parseString(call.Args, "path", "")

	p.logToolCall(global.ToolPlaybookFileDelete, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return nil, fmt.Errorf("%s", "playbook parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}

	if err := p.playbooks.DeleteFile(playbook, path); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"deleted":  true,
	}

	return createJSONResult(result)
}

func (p *Provider) handlePlaybookSearch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	playbook := parseString(call.Args, "playbook", "")
	query := parseString(call.Args, "query", "")
	limit := int(parseFloat64(call.Args, "limit", 0))
	offset := int(parseFloat64(call.Args, "offset", 0))

	p.logToolCall(global.ToolPlaybookSearch, map[string]string{"playbook": playbook, "query": query})

	if query == "" {
		return nil, fmt.Errorf("%s", "query parameter is required")
	}

	items, total, err := p.playbooks.Search(playbook, query, limit, offset)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"items": items,
		"total": total,
		"count": len(items),
	}
	if playbook != "" {
		result["playbook"] = playbook
	}

	return createJSONResult(result)
}
