/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package playbooks provides management of playbook directories and their files.
// Playbooks are reusable knowledge containers that users create and manage.
package playbooks

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/logging"
)

// namePattern validates playbook names (alphanumeric, hyphens, underscores)
var namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// Service provides playbook operations.
type Service struct {
	baseDir   string
	logger    *logging.Logger
	pathMutex sync.Map // per-path locking
}

// Playbook represents a playbook directory.
type Playbook struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// FileItem represents a file within a playbook.
type FileItem struct {
	Playbook   string    `json:"playbook"`
	Path       string    `json:"path"`
	SizeBytes  int64     `json:"size_bytes"`
	ModifiedAt time.Time `json:"modified_at"`
	Summary    string    `json:"summary,omitempty"`
	Content    string    `json:"content,omitempty"`
	// Byte range fields (only set when offset/max_bytes used)
	Offset     int64 `json:"offset,omitempty"`
	TotalBytes int64 `json:"total_bytes,omitempty"`
}

// NewService creates a new playbooks service.
func NewService(baseDir string, logger *logging.Logger) *Service {
	return &Service{
		baseDir:   baseDir,
		logger:    logger,
		pathMutex: sync.Map{},
	}
}

// getPathMutex gets or creates a mutex for a specific path.
func (s *Service) getPathMutex(path string) *sync.Mutex {
	value, _ := s.pathMutex.LoadOrStore(path, &sync.Mutex{})
	return value.(*sync.Mutex)
}

// validateName validates a playbook name.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("playbook name cannot be empty")
	}
	if len(name) > global.DefaultProjectNameMaxLen {
		return fmt.Errorf("playbook name too long (max %d characters)", global.DefaultProjectNameMaxLen)
	}
	if !namePattern.MatchString(name) {
		return fmt.Errorf("playbook name must start with alphanumeric and contain only alphanumeric, hyphens, or underscores")
	}
	return nil
}

// playbookDir returns the directory path for a playbook.
func (s *Service) playbookDir(name string) string {
	return filepath.Join(s.baseDir, name)
}

// validateFilePath validates a file path within a playbook, preventing traversal.
func (s *Service) validateFilePath(playbookName, path string) (string, error) {
	if err := validateName(playbookName); err != nil {
		return "", err
	}

	playbookPath := s.playbookDir(playbookName)

	// Use global path validation
	absPath, err := global.ValidatePathWithinDir(playbookPath, path)
	if err != nil {
		return "", err
	}

	return absPath, nil
}

// List returns all playbooks.
func (s *Service) List() ([]Playbook, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Playbook{}, nil
		}
		return nil, fmt.Errorf("failed to read playbooks directory: %w", err)
	}

	var playbooks []Playbook
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip hidden directories
		if entry.Name()[0] == '.' {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		playbooks = append(playbooks, Playbook{
			Name:      entry.Name(),
			CreatedAt: info.ModTime(),
		})
	}

	s.logger.Debugf("Listed %d playbooks", len(playbooks))
	return playbooks, nil
}

// Create creates a new playbook directory.
func (s *Service) Create(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	playbookPath := s.playbookDir(name)

	mutex := s.getPathMutex(playbookPath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check if already exists
	if global.DirExists(playbookPath) {
		return fmt.Errorf("playbook '%s' already exists", name)
	}

	// Create the directory
	if err := os.MkdirAll(playbookPath, 0755); err != nil {
		return fmt.Errorf("failed to create playbook directory: %w", err)
	}

	s.logger.Infof("Created playbook: %s", name)
	return nil
}

// Rename renames a playbook.
func (s *Service) Rename(name, newName string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if err := validateName(newName); err != nil {
		return err
	}

	oldPath := s.playbookDir(name)
	newPath := s.playbookDir(newName)

	// Lock both paths
	mutex1 := s.getPathMutex(oldPath)
	mutex2 := s.getPathMutex(newPath)
	mutex1.Lock()
	defer mutex1.Unlock()
	mutex2.Lock()
	defer mutex2.Unlock()

	// Check source exists
	if !global.DirExists(oldPath) {
		return fmt.Errorf("playbook '%s' not found", name)
	}

	// Check destination doesn't exist
	if global.DirExists(newPath) {
		return fmt.Errorf("playbook '%s' already exists", newName)
	}

	// Rename
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename playbook: %w", err)
	}

	s.logger.Infof("Renamed playbook: %s -> %s", name, newName)
	return nil
}

// Delete deletes a playbook and all its contents.
func (s *Service) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}

	playbookPath := s.playbookDir(name)

	mutex := s.getPathMutex(playbookPath)
	mutex.Lock()
	defer mutex.Unlock()

	// Check exists
	if !global.DirExists(playbookPath) {
		return fmt.Errorf("playbook '%s' not found", name)
	}

	// Delete recursively
	if err := os.RemoveAll(playbookPath); err != nil {
		return fmt.Errorf("failed to delete playbook: %w", err)
	}

	s.logger.Infof("Deleted playbook: %s", name)
	return nil
}

// Exists checks if a playbook exists.
func (s *Service) Exists(name string) bool {
	if err := validateName(name); err != nil {
		return false
	}
	return global.DirExists(s.playbookDir(name))
}
