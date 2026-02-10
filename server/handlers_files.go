/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/PivotLLM/Maestro/global"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tenebris-tech/x2md/convert"
)

// handleFileCopy handles copying files within and between domains
func (s *Server) handleFileCopy(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse source parameters
	fromSource := mcp.ParseString(request, "from_source", "project")
	fromPlaybook := mcp.ParseString(request, "from_playbook", "")
	fromProject := mcp.ParseString(request, "from_project", "")
	fromPath := mcp.ParseString(request, "from_path", "")

	// Parse destination parameters
	toSource := mcp.ParseString(request, "to_source", "project")
	toPlaybook := mcp.ParseString(request, "to_playbook", "")
	toProject := mcp.ParseString(request, "to_project", "")
	toPath := mcp.ParseString(request, "to_path", "")
	summary := mcp.ParseString(request, "summary", "")

	s.logToolCall(global.ToolFileCopy, map[string]string{
		"from_source": fromSource,
		"from_path":   fromPath,
		"to_source":   toSource,
		"to_path":     toPath,
	})

	// Validate parameters
	if fromPath == "" {
		return mcp.NewToolResultError("from_path parameter is required"), nil
	}
	if toPath == "" {
		return mcp.NewToolResultError("to_path parameter is required"), nil
	}

	// Validate source
	if fromSource != "reference" && fromSource != "playbook" && fromSource != "project" {
		return mcp.NewToolResultError("from_source must be 'reference', 'playbook', or 'project'"), nil
	}

	// Validate destination (reference is read-only)
	if toSource != "playbook" && toSource != "project" {
		return mcp.NewToolResultError("to_source must be 'playbook' or 'project' (reference is read-only)"), nil
	}

	// Read source file (entire file, no byte range)
	var content string
	var err error

	switch fromSource {
	case "reference":
		item, err := s.reference.Get(fromPath, 0, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read source file: %v", err)), nil
		}
		content = item.Content

	case "playbook":
		if fromPlaybook == "" {
			return mcp.NewToolResultError("from_playbook parameter is required when from_source is 'playbook'"), nil
		}
		item, err := s.playbooks.GetFile(fromPlaybook, fromPath, 0, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read source file: %v", err)), nil
		}
		content = item.Content

	case "project":
		if fromProject == "" {
			return mcp.NewToolResultError("from_project parameter is required when from_source is 'project'"), nil
		}
		item, err := s.projects.GetFile(fromProject, fromPath, 0, 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read source file: %v", err)), nil
		}
		content = item.Content
	}

	// Write to destination
	var created bool

	switch toSource {
	case "playbook":
		if toPlaybook == "" {
			return mcp.NewToolResultError("to_playbook parameter is required when to_source is 'playbook'"), nil
		}
		created, err = s.playbooks.PutFile(toPlaybook, toPath, content, summary)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write destination file: %v", err)), nil
		}

	case "project":
		if toProject == "" {
			return mcp.NewToolResultError("to_project parameter is required when to_source is 'project'"), nil
		}
		created, err = s.projects.PutFile(toProject, toPath, content, summary)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write destination file: %v", err)), nil
		}
	}

	result := map[string]interface{}{
		"from_source": fromSource,
		"from_path":   fromPath,
		"to_source":   toSource,
		"to_path":     toPath,
		"copied":      true,
		"created":     created,
	}

	// Add source details if applicable
	if fromSource == "playbook" && fromPlaybook != "" {
		result["from_playbook"] = fromPlaybook
	}
	if fromSource == "project" && fromProject != "" {
		result["from_project"] = fromProject
	}

	// Add destination details
	if toSource == "playbook" && toPlaybook != "" {
		result["to_playbook"] = toPlaybook
	}
	if toSource == "project" && toProject != "" {
		result["to_project"] = toProject
	}

	return createJSONResult(result)
}

// ImportAndConvertResult combines import and optional conversion results
type ImportAndConvertResult struct {
	Project       string `json:"project"`
	Source        string `json:"source"`
	Recursive     bool   `json:"recursive"`
	FilesImported int    `json:"files_imported"`
	LinksImported int    `json:"links_imported"`
	ImportedTo    string `json:"imported_to"`
	// Conversion results (only present if convert=true)
	Converted      *int `json:"converted,omitempty"`
	ConvertSkipped *int `json:"convert_skipped,omitempty"`
	ConvertFailed  *int `json:"convert_failed,omitempty"`
}

// handleFileImport handles importing external files into a project
func (s *Server) handleFileImport(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	source := mcp.ParseString(request, "source", "")
	project := mcp.ParseString(request, "project", "")
	recursive := mcp.ParseBoolean(request, "recursive", false)
	doConvert := mcp.ParseBoolean(request, "convert", false)

	s.logToolCall(global.ToolFileImport, map[string]string{
		"source":    source,
		"project":   project,
		"recursive": fmt.Sprintf("%t", recursive),
		"convert":   fmt.Sprintf("%t", doConvert),
	})

	if source == "" {
		return mcp.NewToolResultError("source parameter is required"), nil
	}
	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	importResult, err := s.projects.ImportFiles(project, source, recursive)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Build result
	result := ImportAndConvertResult{
		Project:       importResult.Project,
		Source:        importResult.Source,
		Recursive:     importResult.Recursive,
		FilesImported: importResult.FilesImported,
		LinksImported: importResult.LinksImported,
		ImportedTo:    importResult.ImportedTo,
	}

	// Run conversion if requested
	if doConvert && importResult.FilesImported > 0 {
		filesDir := s.projects.GetFilesDir(project)
		if filesDir != "" {
			importedPath := filepath.Join(filesDir, importResult.ImportedTo)

			converter := convert.New(
				convert.WithRecursion(true), // Always recursive for imports
				convert.WithSkipExisting(true),
			)

			convertResult, convertErr := converter.Convert(importedPath)
			if convertErr != nil {
				// Log but don't fail - import succeeded
				s.logger.Warnf("Conversion after import failed: %v", convertErr)
			} else {
				converted := convertResult.Converted
				skipped := convertResult.Skipped
				failed := convertResult.Failed
				result.Converted = &converted
				result.ConvertSkipped = &skipped
				result.ConvertFailed = &failed
			}
		}
	}

	return createJSONResult(result)
}
