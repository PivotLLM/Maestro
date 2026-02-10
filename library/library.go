/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package library

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

// Service provides library operations
type Service struct {
	config     *config.Config
	logger     *logging.Logger
	categories map[string]*config.Category
	pathMutex  sync.Map // map[string]*sync.Mutex for per-path locking
}

// Item represents a library item
type Item struct {
	Key        string    `json:"key"`
	Category   string    `json:"category"`
	Path       string    `json:"path"`
	Summary    string    `json:"summary,omitempty"`
	SizeBytes  int64     `json:"size_bytes,omitempty"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
	Content    string    `json:"content,omitempty"`
}

// Metadata represents sidecar metadata
type Metadata struct {
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// parseKey parses a library key into category and path
func (s *Service) parseKey(key string) (string, string, error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid key format: %s (must be category/path)", key)
	}
	return parts[0], parts[1], nil
}

// validateCategory validates a category exists
func (s *Service) validateCategory(category string) (*config.Category, error) {
	cat, exists := s.categories[category]
	if !exists {
		return nil, fmt.Errorf("unknown category: %s", category)
	}
	return cat, nil
}

// resolveFilePath resolves a key to a filesystem path with path traversal prevention
func (s *Service) resolveFilePath(key string) (string, *config.Category, error) {
	category, path, err := s.parseKey(key)
	if err != nil {
		return "", nil, err
	}

	cat, err := s.validateCategory(category)
	if err != nil {
		return "", nil, err
	}

	// Clean the path to prevent traversal
	cleanPath := filepath.Clean(path)

	// Get absolute paths
	absCategoryDir, err := filepath.Abs(cat.Directory)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get absolute category directory: %w", err)
	}

	absFilePath, err := filepath.Abs(filepath.Join(absCategoryDir, cleanPath))
	if err != nil {
		return "", nil, fmt.Errorf("failed to get absolute file path: %w", err)
	}

	// Verify file path is under category directory
	if !strings.HasPrefix(absFilePath, absCategoryDir+string(filepath.Separator)) &&
		absFilePath != absCategoryDir {
		return "", nil, fmt.Errorf("path traversal attempt detected: %s", path)
	}

	return absFilePath, cat, nil
}

// getPathMutex gets or creates a mutex for a specific file path
func (s *Service) getPathMutex(filePath string) *sync.Mutex {
	value, _ := s.pathMutex.LoadOrStore(filePath, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// isValidUTF8File checks if a file contains valid UTF-8
func isValidUTF8File(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if !utf8.Valid(data) {
		return fmt.Errorf("file contains invalid UTF-8 or appears to be binary")
	}

	return nil
}

// loadMetadata loads metadata from sidecar file
func (s *Service) loadMetadata(filePath string) (*Metadata, error) {
	metaPath := filePath + global.MetaSuffix

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil, nil // No metadata file
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata file: %w", err)
	}

	return &meta, nil
}

// saveMetadata saves metadata to sidecar file
func (s *Service) saveMetadata(filePath string, meta *Metadata) error {
	metaPath := filePath + global.MetaSuffix

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Atomic write using temp file
	tempPath := metaPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp metadata file: %w", err)
	}

	if err := os.Rename(tempPath, metaPath); err != nil {
		_ = os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("failed to rename metadata file: %w", err)
	}

	return nil
}

// createItemFromPath creates an Item from a file path
func (s *Service) createItemFromPath(filePath string, category, relativePath string, includeContent bool) (*Item, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("item not found")
		}
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	item := &Item{
		Key:        category + "/" + relativePath,
		Category:   category,
		Path:       relativePath,
		SizeBytes:  stat.Size(),
		ModifiedAt: stat.ModTime(),
	}

	// Load metadata if it exists
	meta, err := s.loadMetadata(filePath)
	if err != nil {
		s.logger.Warnf("Failed to load metadata for %s: %v", filePath, err)
	} else if meta != nil {
		item.Summary = meta.Summary
	}

	// Include content if requested
	if includeContent {
		if err := isValidUTF8File(filePath); err != nil {
			return nil, fmt.Errorf("binary_or_invalid_utf8: %w", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file content: %w", err)
		}
		item.Content = string(content)
	}

	return item, nil
}

// atomicWrite performs an atomic write using a temporary file
func atomicWrite(filePath string, content []byte) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetItem retrieves metadata and optional content for a single key
func (s *Service) GetItem(key string, includeContent bool) (*Item, error) {
	filePath, _, err := s.resolveFilePath(key)
	if err != nil {
		return nil, err
	}

	category, relativePath, _ := s.parseKey(key)

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	item, err := s.createItemFromPath(filePath, category, relativePath, includeContent)
	if err != nil {
		return nil, err
	}

	s.logger.Debugf("Retrieved item: %s", key)
	return item, nil
}

// PutItem creates or overwrites an item with text content
func (s *Service) PutItem(key, content, summary string, overwrite bool) (bool, bool, error) {
	filePath, cat, err := s.resolveFilePath(key)
	if err != nil {
		return false, false, err
	}

	// Check read-only category
	if cat.ReadOnly {
		return false, false, fmt.Errorf("category %s is read-only", cat.Name)
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	_, err = os.Stat(filePath)
	exists := err == nil

	if exists && !overwrite {
		return false, false, fmt.Errorf("file exists and overwrite is false")
	}

	// Write content atomically
	if err := atomicWrite(filePath, []byte(content)); err != nil {
		return false, false, err
	}

	// Create or update metadata
	meta := &Metadata{
		Summary:   summary,
		UpdatedAt: time.Now(),
	}
	if !exists {
		meta.CreatedAt = meta.UpdatedAt
	} else {
		// Preserve existing created time if available
		if existingMeta, err := s.loadMetadata(filePath); err == nil && existingMeta != nil {
			meta.CreatedAt = existingMeta.CreatedAt
		} else {
			meta.CreatedAt = meta.UpdatedAt
		}
	}

	if err := s.saveMetadata(filePath, meta); err != nil {
		s.logger.Warnf("Failed to save metadata for %s: %v", key, err)
	}

	created := !exists
	overwritten := exists

	s.logger.Debugf("Put item: %s (created=%t, overwritten=%t)", key, created, overwritten)
	return created, overwritten, nil
}

// AppendItem appends text to an existing item or creates it if it doesn't exist
func (s *Service) AppendItem(key, content string) (int64, error) {
	filePath, cat, err := s.resolveFilePath(key)
	if err != nil {
		return 0, err
	}

	// Check read-only category
	if cat.ReadOnly {
		return 0, fmt.Errorf("category %s is read-only", cat.Name)
	}

	mutex := s.getPathMutex(filePath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if file exists
	var existingContent []byte
	_, err = os.Stat(filePath)
	exists := err == nil

	if exists {
		// Ensure existing file is valid UTF-8
		if err := isValidUTF8File(filePath); err != nil {
			return 0, fmt.Errorf("binary_or_invalid_utf8: %w", err)
		}

		existingContent, err = os.ReadFile(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to read existing file: %w", err)
		}
	}

	// Append content
	newContent := append(existingContent, []byte(content)...)

	// Write atomically
	if err := atomicWrite(filePath, newContent); err != nil {
		return 0, err
	}

	// Update metadata
	meta := &Metadata{
		UpdatedAt: time.Now(),
	}
	if !exists {
		meta.CreatedAt = meta.UpdatedAt
	} else {
		// Preserve existing metadata
		if existingMeta, err := s.loadMetadata(filePath); err == nil && existingMeta != nil {
			meta.Summary = existingMeta.Summary
			meta.CreatedAt = existingMeta.CreatedAt
		} else {
			meta.CreatedAt = meta.UpdatedAt
		}
	}

	if err := s.saveMetadata(filePath, meta); err != nil {
		s.logger.Warnf("Failed to save metadata for %s: %v", key, err)
	}

	appendedBytes := int64(len(content))
	s.logger.Debugf("Appended to item: %s (%d bytes)", key, appendedBytes)
	return appendedBytes, nil
}
