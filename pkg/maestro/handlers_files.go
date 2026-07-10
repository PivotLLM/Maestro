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

// fileOwner resolves the file operation's domain (source) and its owner name
// (a project or playbook name; empty for reference). writable rejects the
// read-only reference domain. requireName enforces an owner name for the
// project/playbook domains (list and mutating ops); search allows it to be
// empty to mean "search all".
func fileOwner(args map[string]any, writable, requireName bool) (source, name string, err error) {
	source = parseString(args, "source", "project")
	switch source {
	case "project":
		name = parseString(args, "project", "")
		if requireName && name == "" {
			return "", "", fmt.Errorf("project parameter is required when source is 'project'")
		}
	case "playbook":
		name = parseString(args, "playbook", "")
		if requireName && name == "" {
			return "", "", fmt.Errorf("playbook parameter is required when source is 'playbook'")
		}
	case "reference":
		if writable {
			return "", "", fmt.Errorf("source 'reference' is read-only")
		}
	default:
		if writable {
			return "", "", fmt.Errorf("source must be 'project' or 'playbook'")
		}
		return "", "", fmt.Errorf("source must be 'project', 'playbook', or 'reference'")
	}
	return source, name, nil
}

// handleFileList lists files in a project, playbook, or reference domain.
func (p *Provider) handleFileList(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, false, true)
	if err != nil {
		return nil, err
	}
	prefix := parseString(call.Args, "prefix", "")

	p.logToolCall(global.ToolFileList, map[string]string{"source": source, "name": name})

	result := map[string]interface{}{"source": source}
	if name != "" {
		result[source] = name
	}

	switch source {
	case "project":
		items, e := p.projects.ListFiles(name, prefix)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["files"] = items
		result["count"] = len(items)
	case "playbook":
		items, e := p.playbooks.ListFiles(name, prefix)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["files"] = items
		result["count"] = len(items)
	case "reference":
		items, e := p.reference.List(prefix)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["files"] = items
		result["count"] = len(items)
	}

	return createJSONResult(result)
}

// handleFileGet reads a file from a project, playbook, or reference domain.
func (p *Provider) handleFileGet(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, false, true)
	if err != nil {
		return nil, err
	}
	path := parseString(call.Args, "path", "")
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	byteOffset := int64(parseFloat64(call.Args, "byte_offset", 0))
	maxBytes := int64(parseFloat64(call.Args, "max_bytes", 0))

	p.logToolCall(global.ToolFileGet, map[string]string{"source": source, "name": name, "path": path})

	var item interface{}
	switch source {
	case "project":
		item, err = p.projects.GetFile(name, path, byteOffset, maxBytes)
	case "playbook":
		item, err = p.playbooks.GetFile(name, path, byteOffset, maxBytes)
	case "reference":
		item, err = p.reference.Get(path, byteOffset, maxBytes)
	}
	if err != nil {
		return &toolspec.Result{ForLLM: err.Error(), IsError: true}, nil
	}

	return createJSONResult(item)
}

// handleFilePut creates or updates a file in a project or playbook.
func (p *Provider) handleFilePut(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, true, true)
	if err != nil {
		return nil, err
	}
	path := parseString(call.Args, "path", "")
	content := parseString(call.Args, "content", "")
	summary := parseString(call.Args, "summary", "")

	p.logToolCall(global.ToolFilePut, map[string]string{"source": source, "name": name, "path": path})

	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	var created bool
	switch source {
	case "project":
		created, err = p.projects.PutFile(name, path, content, summary)
	case "playbook":
		created, err = p.playbooks.PutFile(name, path, content, summary)
	}
	if err != nil {
		return &toolspec.Result{ForLLM: err.Error(), IsError: true}, nil
	}

	result := map[string]interface{}{"source": source, source: name, "path": path, "created": created}
	return createJSONResult(result)
}

// handleFileAppend appends content to a file in a project or playbook.
func (p *Provider) handleFileAppend(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, true, true)
	if err != nil {
		return nil, err
	}
	path := parseString(call.Args, "path", "")
	content := parseString(call.Args, "content", "")
	summary := parseString(call.Args, "summary", "")

	p.logToolCall(global.ToolFileAppend, map[string]string{"source": source, "name": name, "path": path})

	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if content == "" {
		return nil, fmt.Errorf("%s", "content parameter is required")
	}

	switch source {
	case "project":
		err = p.projects.AppendFile(name, path, content, summary)
	case "playbook":
		err = p.playbooks.AppendFile(name, path, content, summary)
	}
	if err != nil {
		return &toolspec.Result{ForLLM: err.Error(), IsError: true}, nil
	}

	result := map[string]interface{}{"source": source, source: name, "path": path, "success": true}
	return createJSONResult(result)
}

// handleFileEdit edits a file in a project or playbook using search-and-replace.
func (p *Provider) handleFileEdit(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, true, true)
	if err != nil {
		return nil, err
	}
	path := parseString(call.Args, "path", "")
	oldString := parseString(call.Args, "old_string", "")
	newString := parseString(call.Args, "new_string", "")
	replaceAll := parseBool(call.Args, "replace_all", false)

	p.logToolCall(global.ToolFileEdit, map[string]string{"source": source, "name": name, "path": path})

	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}
	if oldString == "" {
		return nil, fmt.Errorf("%s", "old_string parameter is required")
	}
	// new_string can be empty to delete the old_string

	switch source {
	case "project":
		err = p.projects.EditFile(name, path, oldString, newString, replaceAll)
	case "playbook":
		err = p.playbooks.EditFile(name, path, oldString, newString, replaceAll)
	}
	if err != nil {
		return &toolspec.Result{ForLLM: err.Error(), IsError: true}, nil
	}

	result := map[string]interface{}{"source": source, source: name, "path": path, "success": true}
	return createJSONResult(result)
}

// handleFileRename renames or moves a file within a project or playbook.
func (p *Provider) handleFileRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, true, true)
	if err != nil {
		return nil, err
	}
	fromPath := parseString(call.Args, "from_path", "")
	toPath := parseString(call.Args, "to_path", "")

	p.logToolCall(global.ToolFileRename, map[string]string{"source": source, "name": name, "from": fromPath, "to": toPath})

	if fromPath == "" {
		return nil, fmt.Errorf("%s", "from_path parameter is required")
	}
	if toPath == "" {
		return nil, fmt.Errorf("%s", "to_path parameter is required")
	}

	switch source {
	case "project":
		err = p.projects.RenameFile(name, fromPath, toPath)
	case "playbook":
		err = p.playbooks.RenameFile(name, fromPath, toPath)
	}
	if err != nil {
		return &toolspec.Result{ForLLM: err.Error(), IsError: true}, nil
	}

	result := map[string]interface{}{"source": source, source: name, "from": fromPath, "to": toPath, "renamed": true}
	return createJSONResult(result)
}

// handleFileSearch searches files by filename or content in a project,
// playbook, or reference domain. An empty owner name searches all.
func (p *Provider) handleFileSearch(call *toolspec.ToolCall) (*toolspec.Result, error) {
	source, name, err := fileOwner(call.Args, false, false)
	if err != nil {
		return nil, err
	}
	query := parseString(call.Args, "query", "")
	limit := int(parseFloat64(call.Args, "limit", 0))
	offset := int(parseFloat64(call.Args, "offset", 0))

	p.logToolCall(global.ToolFileSearch, map[string]string{"source": source, "name": name, "query": query})

	if query == "" {
		return nil, fmt.Errorf("%s", "query parameter is required")
	}

	result := map[string]interface{}{"source": source}
	if name != "" {
		result[source] = name
	}

	switch source {
	case "project":
		items, total, e := p.projects.SearchFiles(name, query, limit, offset)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["items"] = items
		result["total"] = total
		result["count"] = len(items)
	case "playbook":
		items, total, e := p.playbooks.Search(name, query, limit, offset)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["items"] = items
		result["total"] = total
		result["count"] = len(items)
	case "reference":
		items, total, e := p.reference.Search(query, limit, offset)
		if e != nil {
			return &toolspec.Result{ForLLM: e.Error(), IsError: true}, nil
		}
		result["items"] = items
		result["total"] = total
		result["count"] = len(items)
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
