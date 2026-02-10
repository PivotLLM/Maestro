/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package reference provides read-only access to embedded reference documentation.
package reference

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

// embeddedPrefix is the directory prefix for embedded reference files
const embeddedPrefix = "docs/ai"

// ExternalDir represents an external directory mounted in the reference library
type ExternalDir struct {
	Path  string // Absolute filesystem path
	Mount string // Mount point name (e.g., "user", "standards")
}

// Service provides read-only access to embedded reference files and optional external directories.
type Service struct {
	fs           embed.FS
	prefix       string        // "reference" - the embedded directory prefix
	externalDirs []ExternalDir // external directories mounted in reference library
	logger       *logging.Logger
}

// Item represents a reference file item.
type Item struct {
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at,omitempty"` // Always zero for embedded files
	Content    string    `json:"content,omitempty"`
	// Byte range fields (only set when offset/max_bytes used)
	Offset     int64 `json:"offset,omitempty"`
	TotalBytes int64 `json:"total_bytes,omitempty"`
}

// Option is a functional option for configuring Service
type Option func(*Service)

// WithEmbeddedFS sets the embedded filesystem for reference documentation
func WithEmbeddedFS(efs embed.FS) Option {
	return func(s *Service) {
		s.fs = efs
	}
}

// WithExternalDirs sets the external directories to mount in the reference library
func WithExternalDirs(dirs []ExternalDir) Option {
	return func(s *Service) {
		s.externalDirs = dirs
	}
}

// WithLogger sets the logger for the service
func WithLogger(logger *logging.Logger) Option {
	return func(s *Service) {
		s.logger = logger
	}
}

// NewService creates a new reference service with functional options.
func NewService(opts ...Option) *Service {
	s := &Service{
		prefix: embeddedPrefix,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// validatePath validates and cleans a path, preventing path traversal.
// Returns the cleaned path within the reference prefix.
func (s *Service) validatePath(path string) (string, error) {
	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../") {
		return "", fmt.Errorf("path traversal attempt detected: %s", path)
	}

	// Build the full path within the embedded FS
	fullPath := filepath.Join(s.prefix, cleanPath)

	// Normalize to forward slashes for embed.FS
	fullPath = filepath.ToSlash(fullPath)

	return fullPath, nil
}

// findExternalDir finds which external directory owns a path based on mount prefix.
// Returns the ExternalDir and the relative path within it, or nil if not found.
func (s *Service) findExternalDir(path string) (*ExternalDir, string) {
	cleanPath := filepath.Clean(path)

	for i := range s.externalDirs {
		mount := s.externalDirs[i].Mount
		prefix := mount + "/"

		if strings.HasPrefix(cleanPath, prefix) {
			relPath := strings.TrimPrefix(cleanPath, prefix)
			return &s.externalDirs[i], relPath
		}
		if cleanPath == mount {
			return &s.externalDirs[i], ""
		}
	}

	return nil, ""
}

// isExternal checks if a path refers to an external reference directory.
func (s *Service) isExternal(path string) bool {
	extDir, _ := s.findExternalDir(path)
	return extDir != nil
}

// resolveExternalPath resolves an external path to a filesystem path.
// Returns the absolute filesystem path, the relative path, and the mount name.
func (s *Service) resolveExternalPath(path string) (string, string, string, error) {
	extDir, relPath := s.findExternalDir(path)
	if extDir == nil {
		return "", "", "", fmt.Errorf("path does not match any external reference directory: %s", path)
	}

	// Clean and check for path traversal
	cleanPath := filepath.Clean(relPath)
	if cleanPath != "" && (strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "/../")) {
		return "", "", "", fmt.Errorf("path traversal attempt detected: %s", path)
	}

	// Build absolute path
	absPath := filepath.Join(extDir.Path, cleanPath)

	return absPath, relPath, extDir.Mount, nil
}

// List returns all reference files, optionally filtered by prefix.
func (s *Service) List(prefix string) ([]Item, error) {
	var items []Item

	// Walk the embedded reference directory
	err := fs.WalkDir(s.fs, s.prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == s.prefix {
			return nil
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path from reference/
		relPath, err := filepath.Rel(s.prefix, path)
		if err != nil {
			return nil // Skip if we can't get relative path
		}

		// Normalize to forward slashes
		relPath = filepath.ToSlash(relPath)

		// Apply prefix filter if specified
		if prefix != "" && !strings.HasPrefix(relPath, prefix) {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil // Skip if we can't get info
		}

		items = append(items, Item{
			Path:      relPath,
			SizeBytes: info.Size(),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list embedded reference files: %w", err)
	}

	// Also walk all external reference directories
	for _, extDir := range s.externalDirs {
		// Check if directory exists
		if _, err := os.Stat(extDir.Path); err == nil {
			mountPrefix := extDir.Mount
			dirPath := extDir.Path

			err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Skip errors in external directory
				}

				// Skip the root directory itself
				if path == dirPath {
					return nil
				}

				// Skip directories
				if d.IsDir() {
					return nil
				}

				// Get relative path from external dir
				relPath, err := filepath.Rel(dirPath, path)
				if err != nil {
					return nil // Skip if we can't get relative path
				}

				// Normalize to forward slashes and add mount prefix
				relPath = filepath.ToSlash(relPath)
				fullPath := mountPrefix + "/" + relPath

				// Apply prefix filter if specified
				if prefix != "" && !strings.HasPrefix(fullPath, prefix) {
					return nil
				}

				// Get file info
				info, err := d.Info()
				if err != nil {
					return nil // Skip if we can't get info
				}

				items = append(items, Item{
					Path:       fullPath,
					SizeBytes:  info.Size(),
					ModifiedAt: info.ModTime(),
				})

				return nil
			})
			// Don't fail if external directory walk has errors - just log
			if err != nil {
				s.logger.Warnf("Error walking external reference directory %s: %v", extDir.Mount, err)
			}
		}
	}

	s.logger.Debugf("Listed %d reference files (embedded + external)", len(items))
	return items, nil
}

// Get retrieves a reference file by path with optional byte range.
// If offset is 0 and maxBytes is 0, returns the entire file.
// If maxBytes > 0, returns at most maxBytes starting from offset.
func (s *Service) Get(path string, offset, maxBytes int64) (*Item, error) {
	var content []byte
	var totalBytes int64
	var modTime time.Time

	// Check if this is an external file
	if s.isExternal(path) {
		// Read from external reference directory
		absPath, _, mount, err := s.resolveExternalPath(path)
		if err != nil {
			return nil, err
		}

		// Read the file
		content, err = os.ReadFile(absPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("reference file not found: %s", path)
			}
			return nil, fmt.Errorf("failed to read reference file from %s: %w", mount, err)
		}

		// Get file info
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat reference file: %w", err)
		}
		totalBytes = info.Size()
		modTime = info.ModTime()
	} else {
		// Read from embedded FS
		fullPath, err := s.validatePath(path)
		if err != nil {
			return nil, err
		}

		// Read the file
		content, err = s.fs.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reference file not found: %s", path)
		}

		// Get file info for size
		file, err := s.fs.Open(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open reference file: %s", path)
		}
		defer func(file fs.File) {
			_ = file.Close()
		}(file)

		info, err := file.Stat()
		if err != nil {
			return nil, fmt.Errorf("failed to stat reference file: %s", path)
		}

		totalBytes = info.Size()
		// Embedded files don't have modification times
	}

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

	item := &Item{
		Path:       path,
		SizeBytes:  int64(len(resultContent)),
		ModifiedAt: modTime,
		Content:    resultContent,
		Offset:     resultOffset,
		TotalBytes: totalBytes,
	}

	s.logger.Debugf("Retrieved reference file: %s (offset=%d, bytes=%d, total=%d)", path, resultOffset, len(resultContent), totalBytes)
	return item, nil
}

// Search searches reference files for content matching the query.
func (s *Service) Search(query string, limit, offset int) ([]Item, int, error) {
	if query == "" {
		return nil, 0, fmt.Errorf("search query cannot be empty")
	}

	if limit <= 0 {
		limit = global.DefaultLimit
	}

	var allMatches []Item
	lowerQuery := strings.ToLower(query)

	// Walk and search all files
	err := fs.WalkDir(s.fs, s.prefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(s.prefix, path)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check if path matches
		pathMatch := strings.Contains(strings.ToLower(relPath), lowerQuery)

		// Read content and check for matches
		content, err := s.fs.ReadFile(path)
		if err != nil {
			return nil
		}

		contentMatch := strings.Contains(strings.ToLower(string(content)), lowerQuery)

		if pathMatch || contentMatch {
			info, err := d.Info()
			if err != nil {
				return nil
			}

			allMatches = append(allMatches, Item{
				Path:      relPath,
				SizeBytes: info.Size(),
			})
		}

		return nil
	})

	if err != nil {
		return nil, 0, fmt.Errorf("failed to search embedded reference files: %w", err)
	}

	// Also search all external reference directories
	for _, extDir := range s.externalDirs {
		if _, err := os.Stat(extDir.Path); err == nil {
			mountPrefix := extDir.Mount
			dirPath := extDir.Path

			err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // Skip errors in external directory
				}

				// Skip directories
				if d.IsDir() {
					return nil
				}

				// Get relative path from external dir
				relPath, err := filepath.Rel(dirPath, path)
				if err != nil {
					return nil
				}

				// Add mount prefix
				relPath = filepath.ToSlash(relPath)
				fullPath := mountPrefix + "/" + relPath

				// Check if path matches
				pathMatch := strings.Contains(strings.ToLower(fullPath), lowerQuery)

				// Read content and check for matches
				content, err := os.ReadFile(path)
				if err != nil {
					return nil // Skip files we can't read
				}

				contentMatch := strings.Contains(strings.ToLower(string(content)), lowerQuery)

				if pathMatch || contentMatch {
					info, err := d.Info()
					if err != nil {
						return nil
					}

					allMatches = append(allMatches, Item{
						Path:       fullPath,
						SizeBytes:  info.Size(),
						ModifiedAt: info.ModTime(),
					})
				}

				return nil
			})
			// Don't fail if external directory walk has errors - just log
			if err != nil {
				s.logger.Warnf("Error searching external reference directory %s: %v", extDir.Mount, err)
			}
		}
	}

	// Apply pagination
	total := len(allMatches)

	if offset >= total {
		return []Item{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	results := allMatches[offset:end]

	s.logger.Debugf("Search '%s' found %d total matches, returning %d", query, total, len(results))
	return results, total, nil
}
