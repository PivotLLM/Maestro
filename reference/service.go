/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package reference provides read-only access to embedded reference documentation.
package reference

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
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
//
// An external directory whose mount name equals a built-in top-level name
// (see BuiltinNames) is ignored with a logged warning, so the built-in
// reference files are always reachable.
func NewService(opts ...Option) *Service {
	s := &Service{
		prefix: embeddedPrefix,
	}
	for _, opt := range opts {
		opt(s)
	}

	builtins := topLevelNames(s.fs, s.prefix)
	var kept []ExternalDir
	for _, dir := range s.externalDirs {
		if slices.Contains(builtins, dir.Mount) {
			if s.logger != nil {
				s.logger.Warnf("Ignoring reference directory %s: mount name %q is a built-in reference name", dir.Path, dir.Mount)
			}
			continue
		}
		kept = append(kept, dir)
	}
	s.externalDirs = kept

	return s
}

// BuiltinNames returns the sorted top-level file and folder names of the
// built-in reference library in efs (the filesystem passed to WithEmbeddedFS).
// An external directory mounted under one of these names is ignored, so an
// embedding application should name its mounts around them.
func BuiltinNames(efs fs.FS) []string {
	return topLevelNames(efs, embeddedPrefix)
}

// topLevelNames returns the sorted names of the entries directly under dir in
// fsys, or nil if dir cannot be read.
func topLevelNames(fsys fs.FS, dir string) []string {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// walkExternal calls fn for every regular file in extDir. The directory is
// opened as an os.Root, so symbolic links are followed only while they stay
// inside it: a link that resolves outside the directory, or to a directory, is
// skipped. fsys is the root's filesystem and relPath the file's path in it.
func walkExternal(extDir ExternalDir, fn func(fullPath string, fsys fs.FS, relPath string, info fs.FileInfo)) error {
	root, err := os.OpenRoot(extDir.Path)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()

	fsys := root.FS()
	return fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // Skip unreadable entries and directories
		}
		// fs.Stat follows symbolic links, but only within the root.
		info, err := fs.Stat(fsys, path)
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		fn(extDir.Mount+"/"+path, fsys, path, info)
		return nil
	})
}

// readExternal reads relPath within dir through an os.Root, so neither the
// path nor any symbolic link along it can reach outside dir.
func readExternal(dir, relPath string) ([]byte, fs.FileInfo, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = root.Close() }()

	if relPath == "" {
		relPath = "."
	}
	f, err := root.Open(relPath)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s is not a regular file", relPath)
	}
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, nil, err
	}
	return content, info, nil
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

// resolveExternalPath resolves an external path to its reference directory.
// Returns the directory, the cleaned path relative to it, and the mount name.
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

	if cleanPath == "." {
		cleanPath = ""
	}

	return extDir.Path, cleanPath, extDir.Mount, nil
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
		err := walkExternal(extDir, func(fullPath string, _ fs.FS, _ string, info fs.FileInfo) {
			// Apply prefix filter if specified
			if prefix != "" && !strings.HasPrefix(fullPath, prefix) {
				return
			}
			items = append(items, Item{
				Path:       fullPath,
				SizeBytes:  info.Size(),
				ModifiedAt: info.ModTime(),
			})
		})
		// A missing directory is skipped; other errors are logged, not fatal
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.logger.Warnf("Error walking external reference directory %s: %v", extDir.Mount, err)
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
		dir, relPath, mount, err := s.resolveExternalPath(path)
		if err != nil {
			return nil, err
		}

		// Read the file; nothing outside the directory can be reached.
		var info fs.FileInfo
		content, info, err = readExternal(dir, relPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("reference file not found: %s", path)
			}
			return nil, fmt.Errorf("failed to read reference file %s from %s: %w", path, mount, err)
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
		err := walkExternal(extDir, func(fullPath string, fsys fs.FS, relPath string, info fs.FileInfo) {
			// Check if path matches
			pathMatch := strings.Contains(strings.ToLower(fullPath), lowerQuery)

			// Read content and check for matches
			content, err := fs.ReadFile(fsys, relPath)
			if err != nil {
				return // Skip files we can't read
			}

			contentMatch := strings.Contains(strings.ToLower(string(content)), lowerQuery)

			if pathMatch || contentMatch {
				allMatches = append(allMatches, Item{
					Path:       fullPath,
					SizeBytes:  info.Size(),
					ModifiedAt: info.ModTime(),
				})
			}
		})
		// A missing directory is skipped; other errors are logged, not fatal
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			s.logger.Warnf("Error searching external reference directory %s: %v", extDir.Mount, err)
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
