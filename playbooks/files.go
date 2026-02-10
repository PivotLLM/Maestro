/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package playbooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PivotLLM/Maestro/global"
)

// ListFiles lists files within a playbook, optionally filtered by prefix.
func (s *Service) ListFiles(playbookName, prefix string) ([]FileItem, error) {
	if err := validateName(playbookName); err != nil {
		return nil, err
	}

	playbookPath := s.playbookDir(playbookName)

	// Check playbook exists
	if !global.DirExists(playbookPath) {
		return nil, fmt.Errorf("playbook '%s' not found", playbookName)
	}

	var items []FileItem

	err := filepath.Walk(playbookPath, func(path string, info os.FileInfo, err error) error {
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
		relPath, err := filepath.Rel(playbookPath, path)
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
			Playbook:   playbookName,
			Path:       relPath,
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime(),
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
		return nil, fmt.Errorf("failed to list playbook files: %w", err)
	}

	s.logger.Debugf("Listed %d files in playbook '%s'", len(items), playbookName)
	return items, nil
}

// GetFile retrieves a file from a playbook with optional byte range.
// If offset is 0 and maxBytes is 0, returns the entire file.
// If maxBytes > 0, returns at most maxBytes starting from offset.
func (s *Service) GetFile(playbookName, path string, offset, maxBytes int64) (*FileItem, error) {
	absPath, err := s.validateFilePath(playbookName, path)
	if err != nil {
		return nil, err
	}

	mutex := s.getPathMutex(absPath)
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
		Playbook:   playbookName,
		Path:       path,
		SizeBytes:  int64(len(resultContent)),
		ModifiedAt: info.ModTime(),
		Content:    resultContent,
		Offset:     resultOffset,
		TotalBytes: totalBytes,
	}

	// Load metadata
	meta, err := global.LoadFileMetadata(absPath)
	if err == nil && meta != nil {
		item.Summary = meta.Summary
	}

	s.logger.Debugf("Retrieved file from playbook '%s': %s (offset=%d, bytes=%d, total=%d)", playbookName, path, resultOffset, len(resultContent), totalBytes)
	return item, nil
}

// PutFile creates or overwrites a file in a playbook.
func (s *Service) PutFile(playbookName, path, content, summary string) (bool, error) {
	absPath, err := s.validateFilePath(playbookName, path)
	if err != nil {
		return false, err
	}

	// Ensure playbook exists
	if !s.Exists(playbookName) {
		return false, fmt.Errorf("playbook '%s' not found", playbookName)
	}

	mutex := s.getPathMutex(absPath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	_, err = os.Stat(absPath)
	exists := err == nil

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
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", playbookName, path, err)
	}

	created := !exists
	s.logger.Debugf("Put file in playbook '%s': %s (created=%t)", playbookName, path, created)
	return created, nil
}

// AppendFile appends content to a file in a playbook
func (s *Service) AppendFile(playbookName, path, content, summary string) error {
	absPath, err := s.validateFilePath(playbookName, path)
	if err != nil {
		return err
	}

	// Ensure playbook exists
	if !s.Exists(playbookName) {
		return fmt.Errorf("playbook '%s' not found", playbookName)
	}

	mutex := s.getPathMutex(absPath)
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
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", playbookName, path, err)
	}

	s.logger.Debugf("Appended to file in playbook '%s': %s (existed=%t)", playbookName, path, exists)
	return nil
}

// EditFile performs a search-and-replace edit on a file within a playbook.
func (s *Service) EditFile(playbookName, path, oldString, newString string, replaceAll bool) error {
	if oldString == "" {
		return fmt.Errorf("old_string cannot be empty")
	}
	if oldString == newString {
		return fmt.Errorf("old_string and new_string cannot be identical")
	}

	absPath, err := s.validateFilePath(playbookName, path)
	if err != nil {
		return err
	}

	// Ensure playbook exists
	if !s.Exists(playbookName) {
		return fmt.Errorf("playbook '%s' not found", playbookName)
	}

	mutex := s.getPathMutex(absPath)
	mutex.Lock()
	defer mutex.Unlock()

	// Read current content
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s/%s", playbookName, path)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	content := string(data)

	// Count occurrences
	count := strings.Count(content, oldString)
	if count == 0 {
		return fmt.Errorf("old_string not found in file %s/%s", playbookName, path)
	}
	if count > 1 && !replaceAll {
		return fmt.Errorf("old_string appears %d times in file %s/%s - use replace_all=true to replace all occurrences", count, playbookName, path)
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
		s.logger.Warnf("Failed to save metadata for %s/%s: %v", playbookName, path, err)
	}

	if replaceAll {
		s.logger.Debugf("Edited file in playbook '%s': %s (replaced %d occurrences)", playbookName, path, count)
	} else {
		s.logger.Debugf("Edited file in playbook '%s': %s", playbookName, path)
	}
	return nil
}

// RenameFile renames or moves a file within a playbook.
func (s *Service) RenameFile(playbookName, fromPath, toPath string) error {
	absFromPath, err := s.validateFilePath(playbookName, fromPath)
	if err != nil {
		return err
	}

	absToPath, err := s.validateFilePath(playbookName, toPath)
	if err != nil {
		return err
	}

	// Lock both paths
	mutex1 := s.getPathMutex(absFromPath)
	mutex2 := s.getPathMutex(absToPath)
	mutex1.Lock()
	defer mutex1.Unlock()
	mutex2.Lock()
	defer mutex2.Unlock()

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

	s.logger.Debugf("Renamed file in playbook '%s': %s -> %s", playbookName, fromPath, toPath)
	return nil
}

// DeleteFile deletes a file from a playbook.
func (s *Service) DeleteFile(playbookName, path string) error {
	absPath, err := s.validateFilePath(playbookName, path)
	if err != nil {
		return err
	}

	mutex := s.getPathMutex(absPath)
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

	s.logger.Debugf("Deleted file from playbook '%s': %s", playbookName, path)
	return nil
}

// Search searches for content in playbook files.
// If playbookName is empty, searches all playbooks.
func (s *Service) Search(playbookName, query string, limit, offset int) ([]FileItem, int, error) {
	if query == "" {
		return nil, 0, fmt.Errorf("search query cannot be empty")
	}

	if limit <= 0 {
		limit = global.DefaultLimit
	}

	var playbooks []string
	if playbookName != "" {
		if err := validateName(playbookName); err != nil {
			return nil, 0, err
		}
		if !s.Exists(playbookName) {
			return nil, 0, fmt.Errorf("playbook '%s' not found", playbookName)
		}
		playbooks = []string{playbookName}
	} else {
		// Get all playbooks
		allPlaybooks, err := s.List()
		if err != nil {
			return nil, 0, err
		}
		for _, pb := range allPlaybooks {
			playbooks = append(playbooks, pb.Name)
		}
	}

	var allMatches []FileItem
	lowerQuery := strings.ToLower(query)

	for _, pb := range playbooks {
		playbookPath := s.playbookDir(pb)

		err := filepath.Walk(playbookPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() || strings.HasSuffix(path, global.MetaSuffix) {
				return nil
			}

			relPath, err := filepath.Rel(playbookPath, path)
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
					Playbook:   pb,
					Path:       relPath,
					SizeBytes:  info.Size(),
					ModifiedAt: info.ModTime(),
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
			s.logger.Warnf("Error searching playbook '%s': %v", pb, err)
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
