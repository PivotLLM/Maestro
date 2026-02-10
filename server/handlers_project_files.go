/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package server

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tenebris-tech/x2md/convert"

	"github.com/PivotLLM/Maestro/global"
)

// Project file handlers

func (s *Server) handleProjectFileList(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	prefix := mcp.ParseString(request, "prefix", "")

	s.logToolCall(global.ToolProjectFileList, map[string]string{"project": project})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}

	items, err := s.projects.ListFiles(project, prefix)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"files":   items,
		"count":   len(items),
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileGet(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	byteOffset := int64(mcp.ParseFloat64(request, "byte_offset", 0))
	maxBytes := int64(mcp.ParseFloat64(request, "max_bytes", 0))

	s.logToolCall(global.ToolProjectFileGet, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	item, err := s.projects.GetFile(project, path, byteOffset, maxBytes)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return createJSONResult(item)
}

func (s *Server) handleProjectFilePut(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	content := mcp.ParseString(request, "content", "")
	summary := mcp.ParseString(request, "summary", "")

	s.logToolCall(global.ToolProjectFilePut, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	created, err := s.projects.PutFile(project, path, content, summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"created": created,
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileAppend(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	content := mcp.ParseString(request, "content", "")
	summary := mcp.ParseString(request, "summary", "")

	s.logToolCall(global.ToolProjectFileAppend, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if content == "" {
		return mcp.NewToolResultError("content parameter is required"), nil
	}

	err := s.projects.AppendFile(project, path, content, summary)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"success": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileEdit(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	oldString := mcp.ParseString(request, "old_string", "")
	newString := mcp.ParseString(request, "new_string", "")
	replaceAll := mcp.ParseBoolean(request, "replace_all", false)

	s.logToolCall(global.ToolProjectFileEdit, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}
	if oldString == "" {
		return mcp.NewToolResultError("old_string parameter is required"), nil
	}
	// new_string can be empty to delete the old_string

	err := s.projects.EditFile(project, path, oldString, newString, replaceAll)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"success": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	fromPath := mcp.ParseString(request, "from_path", "")
	toPath := mcp.ParseString(request, "to_path", "")

	s.logToolCall(global.ToolProjectFileRename, map[string]string{"project": project, "from": fromPath, "to": toPath})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if fromPath == "" {
		return mcp.NewToolResultError("from_path parameter is required"), nil
	}
	if toPath == "" {
		return mcp.NewToolResultError("to_path parameter is required"), nil
	}

	if err := s.projects.RenameFile(project, fromPath, toPath); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"from":    fromPath,
		"to":      toPath,
		"renamed": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileDelete(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")

	s.logToolCall(global.ToolProjectFileDelete, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	if err := s.projects.DeleteFile(project, path); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"project": project,
		"path":    path,
		"deleted": true,
	}

	return createJSONResult(result)
}

func (s *Server) handleProjectFileSearch(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	query := mcp.ParseString(request, "query", "")
	limit := int(mcp.ParseFloat64(request, "limit", 0))
	offset := int(mcp.ParseFloat64(request, "offset", 0))

	s.logToolCall(global.ToolProjectFileSearch, map[string]string{"project": project, "query": query})

	if query == "" {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	items, total, err := s.projects.SearchFiles(project, query, limit, offset)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"items": items,
		"total": total,
		"count": len(items),
	}
	if project != "" {
		result["project"] = project
	}

	return createJSONResult(result)
}

// handleProjectFileConvert converts files in a project to Markdown
func (s *Server) handleProjectFileConvert(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	recursive := mcp.ParseBoolean(request, "recursive", false)

	s.logToolCall(global.ToolProjectFileConvert, map[string]string{"project": project, "path": path})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	// Get project files directory
	filesDir := s.projects.GetFilesDir(project)
	if filesDir == "" {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", project)), nil
	}

	// Build full path within project files directory
	fullPath := filepath.Join(filesDir, path)

	// Ensure path is within project files directory (prevent path traversal)
	absFilesDir, err := filepath.Abs(filesDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve files directory: %v", err)), nil
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", err)), nil
	}
	if len(absPath) < len(absFilesDir) || absPath[:len(absFilesDir)] != absFilesDir {
		return mcp.NewToolResultError("path must be within project files directory"), nil
	}

	// Check if path exists and validate type
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("path not found: %s", path)), nil
	}
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to access path: %v", err)), nil
	}

	// Validate path type matches recursive flag
	if recursive {
		if !info.IsDir() {
			return mcp.NewToolResultError("recursive=true requires path to be a directory"), nil
		}
	} else {
		if info.IsDir() {
			return mcp.NewToolResultError("recursive=false requires path to be a file"), nil
		}
	}

	// Create converter with options
	converter := convert.New(
		convert.WithRecursion(recursive),
		convert.WithSkipExisting(true),
	)

	// Run conversion
	result, err := converter.Convert(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("conversion failed: %v", err)), nil
	}

	// Build response
	response := map[string]interface{}{
		"project":   project,
		"path":      path,
		"recursive": recursive,
		"converted": result.Converted,
		"skipped":   result.Skipped,
		"failed":    result.Failed,
	}

	if result.Converted > 0 {
		response["message"] = fmt.Sprintf("Converted %d file(s)", result.Converted)
	} else if result.Skipped > 0 {
		response["message"] = fmt.Sprintf("No files converted (%d skipped)", result.Skipped)
	} else {
		response["message"] = "No files to convert"
	}

	return createJSONResult(response)
}

// handleProjectFileExtract extracts a zip archive within a project's files directory
func (s *Server) handleProjectFileExtract(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := mcp.ParseString(request, "project", "")
	path := mcp.ParseString(request, "path", "")
	overwrite := mcp.ParseBoolean(request, "overwrite", false)
	doConvert := mcp.ParseBoolean(request, "convert", false)

	s.logToolCall(global.ToolProjectFileExtract, map[string]string{
		"project":   project,
		"path":      path,
		"overwrite": fmt.Sprintf("%t", overwrite),
		"convert":   fmt.Sprintf("%t", doConvert),
	})

	if project == "" {
		return mcp.NewToolResultError("project parameter is required"), nil
	}
	if path == "" {
		return mcp.NewToolResultError("path parameter is required"), nil
	}

	// Validate path ends with .zip
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		return mcp.NewToolResultError("path must be a .zip file"), nil
	}

	// Get project files directory
	filesDir := s.projects.GetFilesDir(project)
	if filesDir == "" {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", project)), nil
	}

	// Build full path to zip file
	zipPath := filepath.Join(filesDir, path)

	// Ensure path is within project files directory (prevent path traversal)
	absFilesDir, err := filepath.Abs(filesDir)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve files directory: %v", err)), nil
	}
	absZipPath, err := filepath.Abs(zipPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to resolve path: %v", err)), nil
	}
	if !strings.HasPrefix(absZipPath, absFilesDir+string(filepath.Separator)) {
		return mcp.NewToolResultError("path must be within project files directory"), nil
	}

	// Check zip file exists
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return mcp.NewToolResultError(fmt.Sprintf("zip file not found: %s", path)), nil
	}

	// Determine extraction directory (same name as zip without extension)
	zipBase := filepath.Base(path)
	extractDirName := strings.TrimSuffix(zipBase, filepath.Ext(zipBase))
	extractDir := filepath.Join(filepath.Dir(zipPath), extractDirName)

	// Extract the zip
	extracted, skipped, err := extractZipFile(zipPath, extractDir, overwrite, s.logger)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("extraction failed: %v", err)), nil
	}

	// Sanitize symlinks in extracted directory
	linksRemoved := s.projects.SanitizeSymlinks(extractDir)

	// Build response
	response := map[string]interface{}{
		"project":         project,
		"archive":         path,
		"extracted_to":    filepath.ToSlash(strings.TrimPrefix(extractDir, filesDir+"/")),
		"files_extracted": extracted,
		"files_skipped":   skipped,
		"links_removed":   linksRemoved,
	}

	// Run conversion if requested
	if doConvert && extracted > 0 {
		converter := convert.New(
			convert.WithRecursion(true),
			convert.WithSkipExisting(true),
		)

		convertResult, convertErr := converter.Convert(extractDir)
		if convertErr != nil {
			s.logger.Warnf("Conversion after extraction failed: %v", convertErr)
		} else {
			response["converted"] = convertResult.Converted
			response["convert_skipped"] = convertResult.Skipped
			response["convert_failed"] = convertResult.Failed
		}
	}

	return createJSONResult(response)
}

// extractZipFile extracts a zip archive to the specified directory.
// Returns counts of extracted and skipped files.
func extractZipFile(zipPath, destDir string, overwrite bool, logger interface{ Warnf(string, ...interface{}) }) (int, int, error) {
	// Open the zip file
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	extracted := 0
	skipped := 0

	// Get absolute destination for security checks
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to resolve destination directory: %w", err)
	}

	for _, f := range r.File {
		// Clean and validate the path
		cleanName := filepath.Clean(f.Name)

		// Skip entries that try to escape
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			if logger != nil {
				logger.Warnf("Skipping potentially unsafe path in zip: %s", f.Name)
			}
			skipped++
			continue
		}

		destPath := filepath.Join(destDir, cleanName)

		// Verify the resolved path is within destination directory
		absDestPath, err := filepath.Abs(destPath)
		if err != nil || !strings.HasPrefix(absDestPath, absDestDir+string(filepath.Separator)) {
			if logger != nil {
				logger.Warnf("Skipping path that escapes destination: %s", f.Name)
			}
			skipped++
			continue
		}

		// Handle directories
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return extracted, skipped, fmt.Errorf("failed to create directory %s: %w", cleanName, err)
			}
			continue
		}

		// Check for overwrite
		if !overwrite {
			if _, err := os.Stat(destPath); err == nil {
				skipped++
				continue
			}
		}

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return extracted, skipped, fmt.Errorf("failed to create parent directory for %s: %w", cleanName, err)
		}

		// Extract the file
		if err := extractZipEntry(f, destPath); err != nil {
			return extracted, skipped, fmt.Errorf("failed to extract %s: %w", cleanName, err)
		}

		extracted++
	}

	return extracted, skipped, nil
}

// extractZipEntry extracts a single file from a zip archive
func extractZipEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// Project rename handler
func (s *Server) handleProjectRename(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := mcp.ParseString(request, "name", "")
	newName := mcp.ParseString(request, "new_name", "")

	s.logToolCall(global.ToolProjectRename, map[string]string{"name": name, "new_name": newName})

	if name == "" {
		return mcp.NewToolResultError("name parameter is required"), nil
	}
	if newName == "" {
		return mcp.NewToolResultError("new_name parameter is required"), nil
	}

	if err := s.projects.Rename(name, newName); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]interface{}{
		"from":    name,
		"to":      newName,
		"renamed": true,
	}

	return createJSONResult(result)
}
