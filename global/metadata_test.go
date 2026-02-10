/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFileMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("load existing metadata", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "file.txt")
		metaPath := filePath + MetaSuffix

		// Create metadata file
		metaContent := `{"summary":"Test summary","created_at":"2025-01-15T10:00:00Z","updated_at":"2025-01-15T11:00:00Z"}`
		if err := os.WriteFile(metaPath, []byte(metaContent), 0644); err != nil {
			t.Fatalf("Failed to create metadata file: %v", err)
		}

		meta, err := LoadFileMetadata(filePath)
		if err != nil {
			t.Fatalf("LoadFileMetadata() error = %v", err)
		}
		if meta == nil {
			t.Fatal("LoadFileMetadata() returned nil")
		}
		if meta.Summary != "Test summary" {
			t.Errorf("Summary = %q, want %q", meta.Summary, "Test summary")
		}
	})

	t.Run("no metadata file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "no-meta.txt")

		meta, err := LoadFileMetadata(filePath)
		if err != nil {
			t.Fatalf("LoadFileMetadata() error = %v", err)
		}
		if meta != nil {
			t.Error("LoadFileMetadata() should return nil for non-existent metadata")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "invalid.txt")
		metaPath := filePath + MetaSuffix

		if err := os.WriteFile(metaPath, []byte("not json"), 0644); err != nil {
			t.Fatalf("Failed to create metadata file: %v", err)
		}

		_, err := LoadFileMetadata(filePath)
		if err == nil {
			t.Error("LoadFileMetadata() expected error for invalid JSON")
		}
	})
}

func TestSaveFileMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("save new metadata", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "newfile.txt")
		meta := &FileMetadata{
			Summary:   "New summary",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err := SaveFileMetadata(filePath, meta)
		if err != nil {
			t.Fatalf("SaveFileMetadata() error = %v", err)
		}

		// Verify it can be loaded back
		loaded, err := LoadFileMetadata(filePath)
		if err != nil {
			t.Fatalf("LoadFileMetadata() error = %v", err)
		}
		if loaded.Summary != meta.Summary {
			t.Errorf("Loaded summary = %q, want %q", loaded.Summary, meta.Summary)
		}
	})

	t.Run("overwrite existing metadata", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "existing.txt")

		// Save initial metadata
		initial := &FileMetadata{Summary: "Initial"}
		if err := SaveFileMetadata(filePath, initial); err != nil {
			t.Fatalf("Failed to save initial metadata: %v", err)
		}

		// Save updated metadata
		updated := &FileMetadata{Summary: "Updated"}
		if err := SaveFileMetadata(filePath, updated); err != nil {
			t.Fatalf("SaveFileMetadata() error = %v", err)
		}

		// Verify updated content
		loaded, err := LoadFileMetadata(filePath)
		if err != nil {
			t.Fatalf("LoadFileMetadata() error = %v", err)
		}
		if loaded.Summary != "Updated" {
			t.Errorf("Summary = %q, want %q", loaded.Summary, "Updated")
		}
	})
}

func TestDeleteFileMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "metadata-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("delete existing metadata", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "todelete.txt")
		metaPath := filePath + MetaSuffix

		// Create metadata file
		if err := os.WriteFile(metaPath, []byte(`{"summary":"test"}`), 0644); err != nil {
			t.Fatalf("Failed to create metadata file: %v", err)
		}

		err := DeleteFileMetadata(filePath)
		if err != nil {
			t.Fatalf("DeleteFileMetadata() error = %v", err)
		}

		// Verify it's gone
		if FileExists(metaPath) {
			t.Error("Metadata file should be deleted")
		}
	})

	t.Run("delete non-existent metadata is ok", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "nometa.txt")

		err := DeleteFileMetadata(filePath)
		if err != nil {
			t.Errorf("DeleteFileMetadata() error = %v, want nil for non-existent file", err)
		}
	})
}

func TestNewFileMetadata(t *testing.T) {
	before := time.Now()
	meta := NewFileMetadata("Test summary")
	after := time.Now()

	if meta.Summary != "Test summary" {
		t.Errorf("Summary = %q, want %q", meta.Summary, "Test summary")
	}

	if meta.CreatedAt.Before(before) || meta.CreatedAt.After(after) {
		t.Error("CreatedAt should be within test execution time")
	}

	if meta.UpdatedAt.Before(before) || meta.UpdatedAt.After(after) {
		t.Error("UpdatedAt should be within test execution time")
	}

	if !meta.CreatedAt.Equal(meta.UpdatedAt) {
		t.Error("CreatedAt and UpdatedAt should be equal for new metadata")
	}
}

func TestUpdateFileMetadata(t *testing.T) {
	t.Run("update existing metadata", func(t *testing.T) {
		oldTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		existing := &FileMetadata{
			Summary:   "Old summary",
			CreatedAt: oldTime,
			UpdatedAt: oldTime,
		}

		before := time.Now()
		updated := UpdateFileMetadata(existing, "New summary")
		after := time.Now()

		if updated.Summary != "New summary" {
			t.Errorf("Summary = %q, want %q", updated.Summary, "New summary")
		}

		if !updated.CreatedAt.Equal(oldTime) {
			t.Error("CreatedAt should be preserved from existing metadata")
		}

		if updated.UpdatedAt.Before(before) || updated.UpdatedAt.After(after) {
			t.Error("UpdatedAt should be within test execution time")
		}
	})

	t.Run("update with nil existing creates new", func(t *testing.T) {
		before := time.Now()
		updated := UpdateFileMetadata(nil, "New summary")
		after := time.Now()

		if updated.Summary != "New summary" {
			t.Errorf("Summary = %q, want %q", updated.Summary, "New summary")
		}

		if updated.CreatedAt.Before(before) || updated.CreatedAt.After(after) {
			t.Error("CreatedAt should be within test execution time")
		}
	})
}
