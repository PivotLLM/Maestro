/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// FileMetadata represents sidecar metadata for library/playbook/project files.
// Stored in .meta.json files alongside the actual files.
type FileMetadata struct {
	Summary   string    `json:"summary,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// LoadFileMetadata loads metadata from a sidecar file.
// Returns nil, nil if the metadata file doesn't exist.
func LoadFileMetadata(filePath string) (*FileMetadata, error) {
	metaPath := filePath + MetaSuffix

	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		return nil, nil // No metadata file
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var meta FileMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata file: %w", err)
	}

	return &meta, nil
}

// SaveFileMetadata saves metadata to a sidecar file atomically.
func SaveFileMetadata(filePath string, meta *FileMetadata) error {
	metaPath := filePath + MetaSuffix

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	return AtomicWrite(metaPath, data)
}

// DeleteFileMetadata removes the sidecar metadata file for a given file.
// Returns nil if the metadata file doesn't exist.
func DeleteFileMetadata(filePath string) error {
	metaPath := filePath + MetaSuffix

	err := os.Remove(metaPath)
	if os.IsNotExist(err) {
		return nil // No metadata file to delete
	}
	return err
}

// NewFileMetadata creates a new FileMetadata with current timestamps.
func NewFileMetadata(summary string) *FileMetadata {
	now := time.Now()
	return &FileMetadata{
		Summary:   summary,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// UpdateFileMetadata updates an existing metadata or creates new if nil.
// Preserves CreatedAt if existing metadata is provided.
func UpdateFileMetadata(existing *FileMetadata, summary string) *FileMetadata {
	now := time.Now()
	if existing != nil {
		return &FileMetadata{
			Summary:   summary,
			CreatedAt: existing.CreatedAt,
			UpdatedAt: now,
		}
	}
	return &FileMetadata{
		Summary:   summary,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
