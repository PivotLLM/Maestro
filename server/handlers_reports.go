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

// Report handlers - Read-only domain with controlled write access

func (s *Server) handleReportList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")

	s.logToolCall(global.ToolReportList, map[string]string{"project": project})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	items, err := s.projects.ListReports(project)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"reports": items,
		"count":   len(items),
	}

	return createJSONResult(result)
}

func (s *Server) handleReportRead(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	report := mcp.ParseString(request, "report", "")
	byteOffset := int64(mcp.ParseFloat64(request, "byte_offset", 0))
	maxBytes := int64(mcp.ParseFloat64(request, "max_bytes", 0))

	s.logToolCall(global.ToolReportRead, map[string]string{"project": project, "report": report})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if report == "" {
		return mcp.NewToolResultError("report parameter is required"), nil
	}

	item, err := s.projects.ReadReport(project, report, byteOffset, maxBytes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(item)
}

func (s *Server) handleReportStart(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	title := mcp.ParseString(request, "title", "")
	intro := mcp.ParseString(request, "intro", "")

	s.logToolCall(global.ToolReportStart, map[string]string{"project": project, "title": title})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if title == "" {
		return mcp.NewToolResultError("title parameter is required"), nil
	}

	prefix, err := s.projects.StartReport(project, title, intro)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project":     project,
		"prefix":      prefix,
		"main_report": prefix + "Report.md",
		"message":     "Report session started. Use report_append to add content.",
	}

	return createJSONResult(result)
}

func (s *Server) handleReportAppend(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	content := mcp.ParseString(request, "content", "")
	report := mcp.ParseString(request, "report", "") // Optional - empty means main report

	s.logToolCall(global.ToolReportAppend, map[string]string{"project": project, "report": report})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	err := s.projects.AppendReport(project, content, report)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Get current prefix to return filename info
	prefix, _ := s.projects.GetReportPrefix(project)
	var filename string
	if report == "" {
		filename = prefix + "Report.md"
	} else {
		filename = prefix + report + ".md"
	}

	result := map[string]interface{}{
		"project":       project,
		"report":        filename,
		"bytes_written": len(content),
		"success":       true,
	}

	return createJSONResult(result)
}

func (s *Server) handleReportEnd(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")

	s.logToolCall(global.ToolReportEnd, map[string]string{"project": project})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	// Get prefix and list of reports BEFORE ending the session
	prefix, err := s.projects.GetReportPrefix(project)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// List all reports to identify which ones belong to this session
	allReports, err := s.projects.ListReports(project)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Filter reports that match the current prefix
	var sessionReports []string
	for _, r := range allReports {
		if len(r.Name) >= len(prefix) && r.Name[:len(prefix)] == prefix {
			sessionReports = append(sessionReports, r.Name)
		}
	}

	// Now end the session
	err = s.projects.EndReport(project)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"prefix":  prefix,
		"reports": sessionReports,
		"count":   len(sessionReports),
		"message": "Report session ended. Prefix cleared.",
		"success": true,
	}

	return createJSONResult(result)
}
