/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"github.com/PivotLLM/toolspec"

	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/tenebris-tech/x2md/convert"

	"github.com/PivotLLM/Maestro/global"
)

// handleFileConvert converts files in a project to Markdown. Project files only.
func (p *Provider) handleFileConvert(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	recursive := parseBool(call.Args, "recursive", false)

	p.logToolCall(global.ToolFileConvert, map[string]string{"project": project, "path": path})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}

	// Get project files directory
	filesDir := p.projects.GetFilesDir(project)
	if filesDir == "" {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("project not found: %s", project)), IsError: true}, nil
	}

	// Build full path within project files directory
	fullPath := filepath.Join(filesDir, path)

	// Ensure path is within project files directory (prevent path traversal)
	absFilesDir, err := filepath.Abs(filesDir)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to resolve files directory: %v", err)), IsError: true}, nil
	}
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to resolve path: %v", err)), IsError: true}, nil
	}
	if len(absPath) < len(absFilesDir) || absPath[:len(absFilesDir)] != absFilesDir {
		return nil, fmt.Errorf("%s", "path must be within project files directory")
	}

	// Check if path exists and validate type
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("path not found: %s", path)), IsError: true}, nil
	}
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to access path: %v", err)), IsError: true}, nil
	}

	// Validate path type matches recursive flag
	if recursive {
		if !info.IsDir() {
			return nil, fmt.Errorf("%s", "recursive=true requires path to be a directory")
		}
	} else {
		if info.IsDir() {
			return nil, fmt.Errorf("%s", "recursive=false requires path to be a file")
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
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("conversion failed: %v", err)), IsError: true}, nil
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

// handleFileExtract extracts a zip archive within a project's files directory. Project files only.
func (p *Provider) handleFileExtract(call *toolspec.ToolCall) (*toolspec.Result, error) {
	project := parseString(call.Args, "project", "")
	path := parseString(call.Args, "path", "")
	overwrite := parseBool(call.Args, "overwrite", false)
	doConvert := parseBool(call.Args, "convert", false)

	p.logToolCall(global.ToolFileExtract, map[string]string{
		"project":   project,
		"path":      path,
		"overwrite": fmt.Sprintf("%t", overwrite),
		"convert":   fmt.Sprintf("%t", doConvert),
	})

	if project == "" {
		return nil, fmt.Errorf("%s", "project parameter is required")
	}
	if path == "" {
		return nil, fmt.Errorf("%s", "path parameter is required")
	}

	// Validate path ends with .zip
	if !strings.HasSuffix(strings.ToLower(path), ".zip") {
		return nil, fmt.Errorf("%s", "path must be a .zip file")
	}

	// Get project files directory
	filesDir := p.projects.GetFilesDir(project)
	if filesDir == "" {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("project not found: %s", project)), IsError: true}, nil
	}

	// Build full path to zip file
	zipPath := filepath.Join(filesDir, path)

	// Ensure path is within project files directory (prevent path traversal)
	absFilesDir, err := filepath.Abs(filesDir)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to resolve files directory: %v", err)), IsError: true}, nil
	}
	absZipPath, err := filepath.Abs(zipPath)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("failed to resolve path: %v", err)), IsError: true}, nil
	}
	if !strings.HasPrefix(absZipPath, absFilesDir+string(filepath.Separator)) {
		return nil, fmt.Errorf("%s", "path must be within project files directory")
	}

	// Check zip file exists
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("zip file not found: %s", path)), IsError: true}, nil
	}

	// Determine extraction directory (same name as zip without extension)
	zipBase := filepath.Base(path)
	extractDirName := strings.TrimSuffix(zipBase, filepath.Ext(zipBase))
	extractDir := filepath.Join(filepath.Dir(zipPath), extractDirName)

	// Extract the zip
	extracted, skipped, err := extractZipFile(zipPath, extractDir, overwrite, p.logger)
	if err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(fmt.Sprintf("extraction failed: %v", err)), IsError: true}, nil
	}

	// Sanitize symlinks in extracted directory
	linksRemoved := p.projects.SanitizeSymlinks(extractDir)

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
			p.logger.Warnf("Conversion after extraction failed: %v", convertErr)
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
func (p *Provider) handleProjectRename(call *toolspec.ToolCall) (*toolspec.Result, error) {
	name := parseString(call.Args, "name", "")
	newName := parseString(call.Args, "new_name", "")

	p.logToolCall(global.ToolProjectRename, map[string]string{"name": name, "new_name": newName})

	if name == "" {
		return nil, fmt.Errorf("%s", "name parameter is required")
	}
	if newName == "" {
		return nil, fmt.Errorf("%s", "new_name parameter is required")
	}

	if err := p.projects.Rename(name, newName); err != nil {
		return &toolspec.Result{ForLLM: fmt.Sprint(err.Error()), IsError: true}, nil
	}

	result := map[string]interface{}{
		"from":    name,
		"to":      newName,
		"renamed": true,
	}

	return createJSONResult(result)
}
