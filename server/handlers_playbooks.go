/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/PivotLLM/Maestro/global"
)

// Playbook tool handlers

func (s *Server) handlePlaybookList(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	s.logToolCall(global.ToolPlaybookList, nil)
	playbooks, err := s.playbooks.List()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbooks": playbooks,
		"count":     len(playbooks),
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookCreate(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")

	s.logToolCall(global.ToolPlaybookCreate, map[string]string{"name": name})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	if err := s.playbooks.Create(name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"created":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")
	newName := mcp.ParseString(request, "new_name", "")

	s.logToolCall(global.ToolPlaybookRename, map[string]string{"name": name, "new_name": newName})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}
	if newName == "" {
		return mcp.NewToolResultError("new_name parameter is required"), nil
	}

	if err := s.playbooks.Rename(name, newName); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"from":    name,
		"to":      newName,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookDelete(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")

	s.logToolCall(global.ToolPlaybookDelete, map[string]string{"name": name})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}

	if err := s.playbooks.Delete(name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": name,
		"deleted":  true,
	}

	return createJSONResult(result)
}

// Playbook file handlers

func (s *Server) handlePlaybookFileList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	prefix := mcp.ParseString(request, "prefix", "")

	s.logToolCall(global.ToolPlaybookFileList, map[string]string{"playbook": playbook})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}

	items, err := s.playbooks.ListFiles(playbook, prefix)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"files":    items,
		"count":    len(items),
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookFileGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	path := mcp.ParseString(request, "path", "")
	byteOffset := int64(mcp.ParseFloat64(request, "byte_offset", 0))
	maxBytes := int64(mcp.ParseFloat64(request, "max_bytes", 0))

	s.logToolCall(global.ToolPlaybookFileGet, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	item, err := s.playbooks.GetFile(playbook, path, byteOffset, maxBytes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(item)
}

func (s *Server) handlePlaybookFilePut(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	path := mcp.ParseString(request, "path", "")
	content := mcp.ParseString(request, "content", "")
	summary := mcp.ParseString(request, "summary", "")

	s.logToolCall(global.ToolPlaybookFilePut, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	created, err := s.playbooks.PutFile(playbook, path, content, summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"created":  created,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookFileAppend(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	path := mcp.ParseString(request, "path", "")
	content := mcp.ParseString(request, "content", "")
	summary := mcp.ParseString(request, "summary", "")

	s.logToolCall(global.ToolPlaybookFileAppend, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	err := s.playbooks.AppendFile(playbook, path, content, summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"success":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookFileEdit(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	path := mcp.ParseString(request, "path", "")
	oldString := mcp.ParseString(request, "old_string", "")
	newString := mcp.ParseString(request, "new_string", "")
	replaceAll := mcp.ParseBoolean(request, "replace_all", false)

	s.logToolCall(global.ToolPlaybookFileEdit, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if oldString == "" {
		return mcp.NewToolResultError("old_string parameter is required"), nil
	}
	// new_string can be empty to delete the old_string

	err := s.playbooks.EditFile(playbook, path, oldString, newString, replaceAll)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"success":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookFileRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	fromPath := mcp.ParseString(request, "from_path", "")
	toPath := mcp.ParseString(request, "to_path", "")

	s.logToolCall(global.ToolPlaybookFileRename, map[string]string{"playbook": playbook, "from": fromPath, "to": toPath})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if fromPath == "" {
		return mcp.NewToolResultError("from_path parameter is required"), nil
	}
	if toPath == "" {
		return mcp.NewToolResultError("to_path parameter is required"), nil
	}

	if err := s.playbooks.RenameFile(playbook, fromPath, toPath); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"from":     fromPath,
		"to":       toPath,
		"renamed":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookFileDelete(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	path := mcp.ParseString(request, "path", "")

	s.logToolCall(global.ToolPlaybookFileDelete, map[string]string{"playbook": playbook, "path": path})

	if playbook == "" {
		return mcp.NewToolResultError("playbook parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	if err := s.playbooks.DeleteFile(playbook, path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"playbook": playbook,
		"path":     path,
		"deleted":  true,
	}

	return createJSONResult(result)
}

func (s *Server) handlePlaybookSearch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	playbook := mcp.ParseString(request, "playbook", "")
	query := mcp.ParseString(request, "query", "")
	limit := int(mcp.ParseFloat64(request, "limit", 0))
	offset := int(mcp.ParseFloat64(request, "offset", 0))

	s.logToolCall(global.ToolPlaybookSearch, map[string]string{"playbook": playbook, "query": query})

	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	items, total, err := s.playbooks.Search(playbook, query, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
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
