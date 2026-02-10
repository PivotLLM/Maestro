/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PivotLLM/Maestro/global"
)

// FileItem represents a file within a project's files directory.
type FileItem struct {
	Project    string `json:"project"`
	Path       string `json:"path"`
	SizeBytes  int64  `json:"size_bytes"`
	ModifiedAt string `json:"modified_at"`
	Summary    string `json:"summary,omitempty"`
	Content    string `json:"content,omitempty"`
	// Byte range fields (only set when offset/max_bytes used)
	Offset     int64 `json:"offset,omitempty"`
	TotalBytes int64 `json:"total_bytes,omitempty"`
}

// getFilesDir returns the path to the files directory for a project.
func (s *Service) getFilesDir(project string) string {
	return filepath.Join(s.getProjectDir(project), "files")
}

// validateFilePath validates a file path within a project, preventing traversal.
func (s *Service) validateFilePath(project, path string) (string, error) {
	if err := validateProjectName(project); err != nil {
		return "", err
	}

	filesDir := s.getFilesDir(project)

	// Use global path validation
	absPath, err := global.ValidatePathWithinDir(filesDir, path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// ListFiles lists files within a project, optionally filtered by prefix.
func (s *Service) ListFiles(project, prefix string) ([]FileItem, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	filesDir := s.getFilesDir(project)

	// Check if files directory exists
	if !global.DirExists(filesDir) {
		return []FileItem{}, nil
	}

	var items []FileItem

	err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't read
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip metadata files
		if strings.HasSuffix(path, global.MetaSuffix) {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(filesDir, path)
		if err != nil {
			return nil
		}

		// Normalize to forward slashes
		relPath = filepath.ToSlash(relPath)

		// Apply prefix filter if specified
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		item := FileItem{
			Project:    project,
			Path:       relPath,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		}

		// Load metadata if exists
		meta, err := global.LoadFileMetadata(path)
		if err == nil && meta != nil {
			item.Summary = meta.Summary
		}

		items = append(items, item)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list project files: %w", err)
	}

	s.logger.Debugf("Listed %d files in project '%s'", len(items), project)
	return items, nil
}

// GetFile retrieves a file from a project with optional byte range.
// If offset is 0 and maxBytes is 0, returns the entire file.
// If maxBytes > 0, returns at most maxBytes starting from offset.
func (s *Service) GetFile(project, path string, offset, maxBytes int64) (*FileItem, error) {
	absPath, err := s.validateFilePath(project, path)
	if err != nil {
		return nil, err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.IsDir() {
		return nil, fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check UTF-8
	if err := global.IsValidUTF8File(absPath); err != nil {
		return nil, fmt.Errorf("binary_or_invalid_utf8: %w", err)
	}

	// Read content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	totalBytes := info.Size()

	// Apply byte range if specified
	var resultContent string
	var resultOffset int64

	if maxBytes > 0 {
		// Validate offset
		if offset < 0 {
			offset = 0
		}
		if offset >= int64(len(content)) {
			// Offset beyond file size - return empty content
			resultContent = ""
			resultOffset = offset
		} else {
			end := offset + maxBytes
			if end > int64(len(content)) {
				end = int64(len(content))
			}
			resultContent = string(content[offset:end])
			resultOffset = offset
		}
	} else {
		// No byte range - return entire file
		resultContent = string(content)
		resultOffset = 0
	}

	item := &FileItem{
		Project:    project,
		Path:       path,
		SizeBytes:  int64(len(resultContent)),
		ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		Content:    resultContent,
		Offset:     resultOffset,
		TotalBytes: totalBytes,
	}

	// Load metadata
	meta, err := global.LoadFileMetadata(absPath)
	if err == nil && meta != nil {
		item.Summary = meta.Summary
	}

	s.logger.Debugf("Retrieved file from project '%s': %s (offset=%d, bytes=%d, total=%d)", project, path, resultOffset, len(resultContent), totalBytes)
	return item, nil
}

// PutFile creates or overwrites a file in a project.
func (s *Service) PutFile(project, path, content, summary string) (bool, error) {
	absPath, err := s.validateFilePath(project, path)
	if err != nil {
		return false, err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return false, fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	_, err = os.Stat(absPath)
	exists := err == nil

	// Ensure parent directory exists
	parentDir := filepath.Dir(absPath)
	if err := global.EnsureDir(parentDir); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write content atomically
	if err := global.AtomicWrite(absPath, []byte(content)); err != nil {
		return false, err
	}

	// Update metadata
	var meta *global.FileMetadata
	if exists {
		existingMeta, _ := global.LoadFileMetadata(absPath)
		meta = global.UpdateFileMetadata(existingMeta, summary)
	} else {
		meta = global.NewFileMetadata(summary)
	}

	if err := global.SaveFileMetadata(absPath, meta); err != nil {
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", project, path, err)
	}

	created := !exists
	s.logger.Debugf("Put file in project '%s': %s (created=%t)", project, path, created)
	return created, nil
}

// AppendFile appends content to a file in a project
func (s *Service) AppendFile(project, path, content, summary string) error {
	absPath, err := s.validateFilePath(project, path)
	if err != nil {
		return err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	_, err = os.Stat(absPath)
	exists := err == nil

	// Read existing content if file exists
	var existingContent string
	if exists {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read existing file: %w", err)
		}
		existingContent = string(data)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(absPath)
	if err := global.EnsureDir(parentDir); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Append content
	newContent := existingContent + content

	// Write content atomically
	if err := global.AtomicWrite(absPath, []byte(newContent)); err != nil {
		return err
	}

	// Update metadata
	var meta *global.FileMetadata
	if exists {
		existingMeta, _ := global.LoadFileMetadata(absPath)
		meta = global.UpdateFileMetadata(existingMeta, summary)
	} else {
		meta = global.NewFileMetadata(summary)
	}

	if err := global.SaveFileMetadata(absPath, meta); err != nil {
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", project, path, err)
	}

	s.logger.Debugf("Appended to file in project '%s': %s (existed=%t)", project, path, exists)
	return nil
}

// EditFile performs a search-and-replace edit on a file within a project.
func (s *Service) EditFile(project, path, oldString, newString string, replaceAll bool) error {
	if oldString == "" {
		return fmt.Errorf("old_string cannot be empty")
	}
	if oldString == newString {
		return fmt.Errorf("old_string and new_string cannot be identical")
	}

	absPath, err := s.validateFilePath(project, path)
	if err != nil {
		return err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Read current content
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s/%s", project, path)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// Count occurrences
	count := strings.Count(content, oldString)
	if count == 0 {
		return fmt.Errorf("old_string not found in file %s/%s", project, path)
	}
	if count > 1 && !replaceAll {
		return fmt.Errorf("old_string appears %d times in file %s/%s - use replace_all=true to replace all occurrences", count, project, path)
	}

	// Perform replacement
	var newContent string
	if replaceAll {
		newContent = strings.ReplaceAll(content, oldString, newString)
	} else {
		newContent = strings.Replace(content, oldString, newString, 1)
	}

	// Write updated content atomically
	if err := global.AtomicWrite(absPath, []byte(newContent)); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update metadata (preserve existing summary)
	existingMeta, _ := global.LoadFileMetadata(absPath)
	summary := ""
	if existingMeta != nil {
		summary = existingMeta.Summary
	}
	meta := global.UpdateFileMetadata(existingMeta, summary)
	if err := global.SaveFileMetadata(absPath, meta); err != nil {
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", project, path, err)
	}

	if replaceAll {
		s.logger.Debugf("Edited file in project '%s': %s (replaced %d occurrences)", project, path, count)
	} else {
		s.logger.Debugf("Edited file in project '%s': %s", project, path)
	}
	return nil
}

// RenameFile renames or moves a file within a project.
func (s *Service) RenameFile(project, fromPath, toPath string) error {
	absFromPath, err := s.validateFilePath(project, fromPath)
	if err != nil {
		return err
	}

	absToPath, err := s.validateFilePath(project, toPath)
	if err != nil {
		return err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check source exists
	if !global.FileExists(absFromPath) {
		return fmt.Errorf("source file not found: %s", fromPath)
	}

	// Check destination doesn't exist
	if global.FileExists(absToPath) {
		return fmt.Errorf("destination file already exists: %s", toPath)
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(absToPath)
	if err := global.EnsureDir(destDir); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Rename file
	if err := os.Rename(absFromPath, absToPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Rename metadata file if exists
	metaFromPath := absFromPath + global.MetaSuffix
	metaToPath := absToPath + global.MetaSuffix
	if global.FileExists(metaFromPath) {
		_ = os.Rename(metaFromPath, metaToPath) // Best effort
	}

	s.logger.Debugf("Renamed file in project '%s': %s -> %s", project, fromPath, toPath)
	return nil
}

// DeleteFile deletes a file from a project.
func (s *Service) DeleteFile(project, path string) error {
	absPath, err := s.validateFilePath(project, path)
	if err != nil {
		return err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return fmt.Errorf("project not found: %s", project)
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Check file exists
	if !global.FileExists(absPath) {
		return fmt.Errorf("file not found: %s", path)
	}

	// Delete file
	if err := os.Remove(absPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file if exists
	_ = global.DeleteFileMetadata(absPath)

	s.logger.Debugf("Deleted file from project '%s': %s", project, path)
	return nil
}

// ImportResult contains information about an import operation.
type ImportResult struct {
	Project       string `json:"project"`
	Source        string `json:"source"`
	Recursive     bool   `json:"recursive"`
	FilesImported int    `json:"files_imported"`
	LinksImported int    `json:"links_imported"`
	LinksRemoved  int    `json:"links_removed,omitempty"` // Symlinks removed for escaping base directory
	ImportedTo    string `json:"imported_to"`
}

// ImportFiles imports external files into a project's files/imported/ directory.
// This bypasses the chroot to allow importing from anywhere on the filesystem.
// The source can be a file or directory. If recursive is true, directories are
// imported recursively preserving their structure. Symlinks are preserved as symlinks.
func (s *Service) ImportFiles(project, source string, recursive bool) (*ImportResult, error) {
	if err := validateProjectName(project); err != nil {
		return nil, err
	}

	// Verify project exists
	if !s.ProjectExists(project) {
		return nil, fmt.Errorf("project not found: %s", project)
	}

	// Use Lstat to not follow symlinks for the source itself
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("source path not found: %s", source)
		}
		return nil, fmt.Errorf("failed to access source: %w", err)
	}

	// Create base imported directory
	baseImportedDir := filepath.Join(s.getFilesDir(project), "imported")
	if err := global.EnsureDir(baseImportedDir); err != nil {
		return nil, fmt.Errorf("failed to create imported directory: %w", err)
	}

	// Determine the target directory - preserve source name for directories
	sourceName := filepath.Base(source)
	var targetDir string
	var importedTo string

	if sourceInfo.Mode()&os.ModeSymlink != 0 || !sourceInfo.IsDir() {
		// For files/symlinks, put directly in imported/
		targetDir = baseImportedDir
		importedTo = filepath.ToSlash(filepath.Join("imported", sourceName))
	} else {
		// For directories, preserve the directory name: imported/<dirname>/...
		targetDir = filepath.Join(baseImportedDir, sourceName)
		importedTo = filepath.ToSlash(filepath.Join("imported", sourceName))
		if err := global.EnsureDir(targetDir); err != nil {
			return nil, fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	result := &ImportResult{
		Project:    project,
		Source:     source,
		Recursive:  recursive,
		ImportedTo: importedTo,
	}

	mutex := s.getProjectMutex(project)
	mutex.Lock()
	defer mutex.Unlock()

	// Handle symlink to directory or file
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		// Source itself is a symlink - copy it as a symlink
		destPath := filepath.Join(targetDir, sourceName)
		if err := copySymlink(source, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy symlink: %w", err)
		}
		result.LinksImported = 1
		s.logger.Infof("Imported 1 symlink into project '%s' from '%s'", project, source)
		return result, nil
	}

	if sourceInfo.IsDir() {
		// Import directory
		if !recursive {
			return nil, fmt.Errorf("source is a directory but recursive is false")
		}

		// Walk the source directory without following symlinks into directories
		err := walkNoFollow(source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip files we can't read
			}

			// Skip the root directory itself
			if path == source {
				return nil
			}

			// Get relative path from source
			relPath, err := filepath.Rel(source, path)
			if err != nil {
				return nil
			}

			// Build destination path (inside targetDir which already includes source name)
			destPath := filepath.Join(targetDir, relPath)

			// Check if it's a symlink
			if info.Mode()&os.ModeSymlink != 0 {
				// Ensure destination directory exists
				destDir := filepath.Dir(destPath)
				if err := global.EnsureDir(destDir); err != nil {
					s.logger.Warnf("Failed to create directory for import: %v", err)
					return nil
				}

				// Copy symlink
				if err := copySymlink(path, destPath); err != nil {
					s.logger.Warnf("Failed to copy symlink %s: %v", path, err)
					return nil
				}

				result.LinksImported++
				return nil
			}

			// Skip directories (they'll be created as needed)
			if info.IsDir() {
				return nil
			}

			// Ensure destination directory exists
			destDir := filepath.Dir(destPath)
			if err := global.EnsureDir(destDir); err != nil {
				s.logger.Warnf("Failed to create directory for import: %v", err)
				return nil
			}

			// Copy file
			if err := copyFile(path, destPath); err != nil {
				s.logger.Warnf("Failed to copy file %s: %v", path, err)
				return nil
			}

			result.FilesImported++
			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk source directory: %w", err)
		}
	} else {
		// Import single file
		destPath := filepath.Join(targetDir, sourceName)
		if err := copyFile(source, destPath); err != nil {
			return nil, fmt.Errorf("failed to copy file: %w", err)
		}

		result.FilesImported = 1
	}

	// Sanitize symlinks - remove any that escape the imported directory
	importedFullPath := filepath.Join(s.getFilesDir(project), "imported")
	result.LinksRemoved = sanitizeSymlinks(importedFullPath, s.logger)

	if result.LinksRemoved > 0 {
		result.LinksImported -= result.LinksRemoved
		s.logger.Infof("Imported %d files and %d symlinks into project '%s' at '%s' (%d unsafe symlinks removed)",
			result.FilesImported, result.LinksImported, project, importedTo, result.LinksRemoved)
	} else {
		s.logger.Infof("Imported %d files and %d symlinks into project '%s' at '%s'",
			result.FilesImported, result.LinksImported, project, importedTo)
	}
	return result, nil
}

// walkNoFollow walks a directory tree without following symlinks into directories.
// It uses Lstat instead of Stat so symlinks are reported as symlinks.
func walkNoFollow(root string, walkFn filepath.WalkFunc) error {
	info, err := os.Lstat(root)
	if err != nil {
		return walkFn(root, nil, err)
	}
	return walkNoFollowRecursive(root, info, walkFn)
}

func walkNoFollowRecursive(path string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	err := walkFn(path, info, nil)
	if err != nil {
		if info.IsDir() && err == filepath.SkipDir {
			return nil
		}
		return err
	}

	// Don't recurse into symlinks (they might be circular or point elsewhere)
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	if !info.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return walkFn(path, info, err)
	}

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		childInfo, err := os.Lstat(childPath)
		if err != nil {
			if err := walkFn(childPath, nil, err); err != nil && err != filepath.SkipDir {
				return err
			}
			continue
		}
		if err := walkNoFollowRecursive(childPath, childInfo, walkFn); err != nil {
			return err
		}
	}
	return nil
}

// copySymlink recreates a symlink at dst pointing to the same target as src.
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return err
	}

	// Remove existing destination if it exists
	_ = os.Remove(dst)

	return os.Symlink(target, dst)
}

// sanitizeSymlinks scans a directory for symlinks that escape the allowed base directory
// and removes them. Returns the count of removed symlinks.
// This prevents symlink attacks where imported symlinks point outside the project.
func sanitizeSymlinks(baseDir string, logger interface{ Warnf(string, ...interface{}) }) int {
	removed := 0

	// Get absolute base directory for comparison
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return 0
	}

	_ = walkNoFollow(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}

		// Only check symlinks
		if info.Mode()&os.ModeSymlink == 0 {
			return nil
		}

		// Read the symlink target
		target, err := os.Readlink(path)
		if err != nil {
			return nil
		}

		// Resolve the target path
		var resolvedTarget string
		if filepath.IsAbs(target) {
			// Absolute symlinks always escape
			resolvedTarget = target
		} else {
			// Relative symlink - resolve from symlink's directory
			symlinkDir := filepath.Dir(path)
			resolvedTarget = filepath.Clean(filepath.Join(symlinkDir, target))
		}

		// Get absolute path of resolved target
		absTarget, err := filepath.Abs(resolvedTarget)
		if err != nil {
			// Can't resolve - remove to be safe
			if logger != nil {
				logger.Warnf("Removing symlink with unresolvable target: %s -> %s", path, target)
			}
			_ = os.Remove(path)
			removed++
			return nil
		}

		// Check if target is within base directory
		if !strings.HasPrefix(absTarget, absBaseDir+string(filepath.Separator)) && absTarget != absBaseDir {
			// Symlink escapes base directory - remove it
			if logger != nil {
				logger.Warnf("Removing symlink that escapes base directory: %s -> %s (resolves to %s)", path, target, absTarget)
			}
			_ = os.Remove(path)
			removed++
		}

		return nil
	})

	return removed
}

// SanitizeSymlinks is a public wrapper for sanitizeSymlinks that can be called by handlers.
// It scans the given directory for symlinks that escape and removes them.
func (s *Service) SanitizeSymlinks(dir string) int {
	return sanitizeSymlinks(dir, s.logger)
}

// copyFile copies a file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	return err
}

// SearchFiles searches for content in project files.
// If project is empty, searches all projects.
func (s *Service) SearchFiles(project, query string, limit, offset int) ([]FileItem, int, error) {
	if query == "" {
		return nil, 0, fmt.Errorf("search query cannot be empty")
	}

	if limit <= 0 {
		limit = global.DefaultLimit
	}

	var targets []string

	if project != "" {
		if err := validateProjectName(project); err != nil {
			return nil, 0, err
		}
		if !s.ProjectExists(project) {
			return nil, 0, fmt.Errorf("project not found: %s", project)
		}
		targets = append(targets, project)
	} else {
		// Get all projects
		result, err := s.List("", 0, 0)
		if err != nil {
			return nil, 0, err
		}
		for _, proj := range result.Projects {
			targets = append(targets, proj.Name)
		}
	}

	var allMatches []FileItem
	lowerQuery := strings.ToLower(query)

	for _, targetProject := range targets {
		filesDir := s.getFilesDir(targetProject)

		if !global.DirExists(filesDir) {
			continue
		}

		err := filepath.Walk(filesDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() || strings.HasSuffix(path, global.MetaSuffix) {
				return nil
			}

			relPath, err := filepath.Rel(filesDir, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			// Check path match
			pathMatch := strings.Contains(strings.ToLower(relPath), lowerQuery)

			// Read and check content
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			contentMatch := strings.Contains(strings.ToLower(string(content)), lowerQuery)

			if pathMatch || contentMatch {
				item := FileItem{
					Project:    targetProject,
					Path:       relPath,
					SizeBytes:  info.Size(),
					ModifiedAt: info.ModTime().Format("2006-01-02T15:04:05Z07:00"),
				}

				// Load metadata
				meta, _ := global.LoadFileMetadata(path)
				if meta != nil {
					item.Summary = meta.Summary
				}

				allMatches = append(allMatches, item)
			}

			return nil
		})

		if err != nil {
			s.logger.Warnf("Error searching project '%s': %v", targetProject, err)
		}
	}

	// Apply pagination
	total := len(allMatches)

	if offset >= total {
		return []FileItem{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	results := allMatches[offset:end]

	s.logger.Debugf("Search '%s' found %d total matches, returning %d", query, total, len(results))
	return results, total, nil
}
