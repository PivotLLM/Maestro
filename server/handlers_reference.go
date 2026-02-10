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

// Reference tool handlers (read-only, embedded)

func (s *Server) handleReferenceList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prefix := mcp.ParseString(request, "prefix", "")

	s.logToolCall(global.ToolReferenceList, map[string]string{"prefix": prefix})

	items, err := s.reference.List(prefix)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"items": items,
		"count": len(items),
	}

	return createJSONResult(result)
}

func (s *Server) handleReferenceGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := mcp.ParseString(request, "path", "")
	byteOffset := int64(mcp.ParseFloat64(request, "byte_offset", 0))
	maxBytes := int64(mcp.ParseFloat64(request, "max_bytes", 0))

	s.logToolCall(global.ToolReferenceGet, map[string]string{"path": path})

	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	item, err := s.reference.Get(path, byteOffset, maxBytes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(item)
}

func (s *Server) handleReferenceSearch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := mcp.ParseString(request, "query", "")
	limit := int(mcp.ParseFloat64(request, "limit", 0))
	offset := int(mcp.ParseFloat64(request, "offset", 0))

	s.logToolCall(global.ToolReferenceSearch, map[string]string{"query": query})

	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	items, total, err := s.reference.Search(query, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"items": items,
		"total": total,
		"count": len(items),
	}

	return createJSONResult(result)
}
