/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"fmt"
	"path/filepath"

	"github.com/PivotLLM/Maestro/global"
	"github.com/tenebris-tech/x2md/convert"
)

// handleFileCopy handles copying files within and between domains
func (p *Provider) handleFileCopy(call *toolspec.ToolCall) (*toolspec.Result, error) {
	// Parse source parameters
	fromSource := parseString(call.Args, "from_source", "project")
	fromPlaybook := parseString(call.Args, "from_playbook", "")
	fromProject := parseString(call.Args, "from_project", "")
	fromPath := parseString(call.Args, "from_path", "")

	// Parse destination parameters
	toSource := parseString(call.Args, "to_source", "project")
	toPlaybook := parseString(call.Args, "to_playbook", "")
	toProject := parseString(call.Args, "to_project", "")
	toPath := parseString(call.Args, "to_path", "")
	summary := parseString(call.Args, "summary", "")

	p.logToolCall(global.ToolFileCopy, map[string]string{
		"from_source": fromSource,
		"from_path":   fromPath,
		"to_source":   toSource,
		"to_path":     toPath,
	})

	// Validate parameters
	if fromPath == "" {
		return nil, fmt.Errorf("%s", "from_path parameter is required")
	}
	if toPath == "" {
		return nil, fmt.Errorf("%s", "to_path parameter is required")
	}

	// Validate source
	if fromSource != "reference" && fromSource != "playbook" && fromSource != "project" {
		return nil, fmt.Errorf("%s", "from_source must be 'reference', 'playbook', or 'project'")
	}

	// Validate destination (reference is read-only)
	if toSource != "playbook" && toSource != "project" {
		return &toolspec.Result{ForLLM: fmt.Sprint("to_source must be 'playbook' or 'project' (reference is read-only)"), IsError: true}, nil
	}

	// Read source file (entire file, no byte range)
	var content string
	var err error

	switch fromSource {
	case "reference":
		item, err := p.reference.Get(fromPath, 0, 0)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to read source file: %v", err)), IsError: true}, nil
		}
		content = item.Content

	case "playbook":
		if fromPlaybook == "" {
			return nil, fmt.Errorf("%s", "from_playbook parameter is required when from_source is 'playbook'")
		}
		item, err := p.playbooks.GetFile(fromPlaybook, fromPath, 0, 0)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to read source file: %v", err)), IsError: true}, nil
		}
		content = item.Content

	case "project":
		if fromProject == "" {
			return nil, fmt.Errorf("%s", "from_project parameter is required when from_source is 'project'")
		}
		item, err := p.projects.GetFile(fromProject, fromPath, 0, 0)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to read source file: %v", err)), IsError: true}, nil
		}
		content = item.Content
	}

	// Write to destination
	var created bool

	switch toSource {
	case "playbook":
		if toPlaybook == "" {
			return nil, fmt.Errorf("%s", "to_playbook parameter is required when to_source is 'playbook'")
		}
		created, err = p.playbooks.PutFile(toPlaybook, toPath, content, summary)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to write destination file: %v", err)), IsError: true}, nil
		}

	case "project":
		if toProject == "" {
			return nil, fmt.Errorf("%s", "to_project parameter is required when to_source is 'project'")
		}
		created, err = p.projects.PutFile(toProject, toPath, content, summary)
		if err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to write destination file: %v", err)), IsError: true}, nil
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

// handleFileDelete deletes a file from a project or playbook domain.
func (p *Provider) handleFileDelete(call *toolspec.ToolCall) (*toolspec.Result, error) {
	path := parseString(call.Args, "path", "")
	source := parseString(call.Args, "source", "project")
	project := parseString(call.Args, "project", "")
	playbook := parseString(call.Args, "playbook", "")

	p.logToolCall(global.ToolFileDelete, map[string]string{"path": path, "source": source})

	if path == "" {
		return nil, fmt.Errorf("%s", "path is required")
	}

	result := map[string]interface{}{
		"path":    path,
		"source":  source,
		"deleted": true,
	}

	switch source {
	case "project", "":
		if project == "" {
			return nil, fmt.Errorf("%s", "project is required when source is 'project'")
		}
		if err := p.projects.DeleteFile(project, path); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result["project"] = project
	case "playbook":
		if playbook == "" {
			return nil, fmt.Errorf("%s", "playbook is required when source is 'playbook'")
		}
		if err := p.playbooks.DeleteFile(playbook, path); err != nil {
			return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
		}
		result["playbook"] = playbook
	default:
		return &toolspec.Result{ForLLM: fmt.Sprint("source must be 'project' or 'playbook' (reference is read-only)"), IsError: true}, nil
	}

	return createJSONResult(result)
}

// handleFileImport handles importing external files into a project
func (p *Provider) handleFileImport(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source := parseString(call.Args, "source", "")
	project := parseString(call.Args, "project", "")
	recursive := parseBool(call.Args, "recursive", false)
	doConvert := parseBool(call.Args, "convert", false)

	p.logToolCall(global.ToolFileImport, map[string]string{
		"source":    source,
		"project":   project,
		"recursive": fmt.Sprintf("%t", recursive),
		"convert":   fmt.Sprintf("%t", doConvert),
	})

	if source == "" {
		return nil, fmt.Errorf("%s", "source parameter is required")
	}
	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}

	importResult, err := p.projects.ImportFiles(project, source, recursive)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
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
		filesDir := p.projects.GetFilesDir(project)
		if filesDir != "" {
			importedPath := filepath.Join(filesDir, importResult.ImportedTo)

			converter := convert.New(
				convert.WithRecursion(true), // Always recursive for imports
				convert.WithSkipExisting(true),
			)

			convertResult, convertErr := converter.Convert(importedPath)
			if convertErr != nil {
				// Log but don't fail - import succeeded
				p.logger.Warnf("Conversion after import failed: %v", convertErr)
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
