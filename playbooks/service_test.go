/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package playbooks

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/logging"
)

func createTestLogger(t *testing.T) *logging.Logger {
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	if err != nil {
		t.Fatalf("Failed to create temp log file: %v", err)
	}
	_ = tmpFile.Close()

	logger, err := logging.New(tmpFile.Name())
	if err != nil {
		_ = os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create logger: %v", err)
	}

	t.Cleanup(func() {
		_ = logger.Close()
		_ = os.Remove(tmpFile.Name())
	})

	return logger
}

func createTestService(t *testing.T) *Service {
	tmpDir, err := os.MkdirTemp("", "playbooks-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	logger := createTestLogger(t)
	return NewService(tmpDir, logger)
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "test", false},
		{"valid with hyphen", "test-playbook", false},
		{"valid with underscore", "test_playbook", false},
		{"valid with number", "test123", false},
		{"valid alphanumeric", "Test123", false},
		{"empty", "", true},
		{"starts with hyphen", "-test", true},
		{"starts with underscore", "_test", true},
		{"contains space", "test playbook", true},
		{"contains dot", "test.playbook", true},
		{"contains slash", "test/playbook", true},
		{"too long", string(make([]byte, 257)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCreateAndList(t *testing.T) {
	svc := createTestService(t)

	// List should be empty initially
	playbooks, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(playbooks) != 0 {
		t.Errorf("List() returned %d items, want 0", len(playbooks))
	}

	// Create a playbook
	err = svc.Create("test-playbook")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// List should have one item
	playbooks, err = svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(playbooks) != 1 {
		t.Fatalf("List() returned %d items, want 1", len(playbooks))
	}
	if playbooks[0].Name != "test-playbook" {
		t.Errorf("playbook name = %q, want %q", playbooks[0].Name, "test-playbook")
	}

	// Creating duplicate should fail
	err = svc.Create("test-playbook")
	if err == nil {
		t.Error("Create() expected error for duplicate")
	}
}

func TestExists(t *testing.T) {
	svc := createTestService(t)

	// Should not exist
	if svc.Exists("nonexistent") {
		t.Error("Exists() should return false for nonexistent playbook")
	}

	// Create and verify exists
	if err := svc.Create("test"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !svc.Exists("test") {
		t.Error("Exists() should return true after Create()")
	}
}

func TestRename(t *testing.T) {
	svc := createTestService(t)

	// Create a playbook
	if err := svc.Create("original"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Rename it
	err := svc.Rename("original", "renamed")
	if err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// Original should not exist
	if svc.Exists("original") {
		t.Error("Original playbook should not exist after rename")
	}

	// Renamed should exist
	if !svc.Exists("renamed") {
		t.Error("Renamed playbook should exist")
	}

	// Rename nonexistent should fail
	err = svc.Rename("nonexistent", "other")
	if err == nil {
		t.Error("Rename() expected error for nonexistent source")
	}

	// Create another and try to rename to existing name
	if err := svc.Create("another"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	err = svc.Rename("another", "renamed")
	if err == nil {
		t.Error("Rename() expected error when destination exists")
	}
}

func TestDelete(t *testing.T) {
	svc := createTestService(t)

	// Create a playbook
	if err := svc.Create("to-delete"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete it
	err := svc.Delete("to-delete")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Should not exist anymore
	if svc.Exists("to-delete") {
		t.Error("Playbook should not exist after Delete()")
	}

	// Delete nonexistent should fail
	err = svc.Delete("nonexistent")
	if err == nil {
		t.Error("Delete() expected error for nonexistent playbook")
	}
}

func TestValidateFilePath(t *testing.T) {
	svc := createTestService(t)

	// Create a playbook for testing
	if err := svc.Create("test"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	tests := []struct {
		name         string
		playbookName string
		path         string
		wantErr      bool
	}{
		{"valid simple", "test", "file.txt", false},
		{"valid nested", "test", "subdir/file.txt", false},
		{"path traversal ..", "test", "../outside.txt", true},
		{"path traversal nested", "test", "subdir/../../outside.txt", true},
		{"invalid playbook name", "", "file.txt", true},
		// Note: validateFilePath only validates name format and path safety,
		// it doesn't check if the playbook exists (that's done in callers)
		{"valid name format", "nonexistent", "file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.validateFilePath(tt.playbookName, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateFilePath(%q, %q) error = %v, wantErr %v",
					tt.playbookName, tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestFileOperations(t *testing.T) {
	svc := createTestService(t)

	// Create a playbook
	if err := svc.Create("files-test"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("put and get file", func(t *testing.T) {
		created, err := svc.PutFile("files-test", "test.txt", "Hello World", "Test file")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}
		if !created {
			t.Error("PutFile() should return created=true for new file")
		}

		item, err := svc.GetFile("files-test", "test.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Hello World" {
			t.Errorf("Content = %q, want %q", item.Content, "Hello World")
		}
		if item.Summary != "Test file" {
			t.Errorf("Summary = %q, want %q", item.Summary, "Test file")
		}
	})

	t.Run("update existing file", func(t *testing.T) {
		created, err := svc.PutFile("files-test", "test.txt", "Updated content", "Updated summary")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}
		if created {
			t.Error("PutFile() should return created=false for existing file")
		}

		item, err := svc.GetFile("files-test", "test.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Updated content" {
			t.Errorf("Content = %q, want %q", item.Content, "Updated content")
		}
	})

	t.Run("nested file", func(t *testing.T) {
		_, err := svc.PutFile("files-test", "subdir/nested.txt", "Nested content", "")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}

		item, err := svc.GetFile("files-test", "subdir/nested.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Nested content" {
			t.Errorf("Content = %q, want %q", item.Content, "Nested content")
		}
	})

	t.Run("list files", func(t *testing.T) {
		items, err := svc.ListFiles("files-test", "")
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(items) != 2 {
			t.Errorf("ListFiles() returned %d items, want 2", len(items))
		}
	})

	t.Run("list files with prefix", func(t *testing.T) {
		items, err := svc.ListFiles("files-test", "subdir")
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(items) != 1 {
			t.Errorf("ListFiles() with prefix returned %d items, want 1", len(items))
		}
	})

	t.Run("rename file", func(t *testing.T) {
		err := svc.RenameFile("files-test", "test.txt", "renamed.txt")
		if err != nil {
			t.Fatalf("RenameFile() error = %v", err)
		}

		// Old file should not exist
		_, err = svc.GetFile("files-test", "test.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for renamed file's old path")
		}

		// New file should exist
		item, err := svc.GetFile("files-test", "renamed.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Updated content" {
			t.Errorf("Content = %q, want %q", item.Content, "Updated content")
		}
	})

	t.Run("delete file", func(t *testing.T) {
		err := svc.DeleteFile("files-test", "renamed.txt")
		if err != nil {
			t.Fatalf("DeleteFile() error = %v", err)
		}

		// File should not exist
		_, err = svc.GetFile("files-test", "renamed.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for deleted file")
		}
	})

	t.Run("get nonexistent file", func(t *testing.T) {
		_, err := svc.GetFile("files-test", "nonexistent.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for nonexistent file")
		}
	})

	t.Run("byte range", func(t *testing.T) {
		// Create a file with known content
		_, err := svc.PutFile("files-test", "range-test.txt", "Hello World Byte Range", "")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}

		// Get first 5 bytes
		item, err := svc.GetFile("files-test", "range-test.txt", 0, 5)
		if err != nil {
			t.Fatalf("GetFile() with byte range error = %v", err)
		}
		if item.Content != "Hello" {
			t.Errorf("Content = %q, want %q", item.Content, "Hello")
		}
		if item.Offset != 0 {
			t.Errorf("Offset = %d, want 0", item.Offset)
		}
		if item.TotalBytes != 22 {
			t.Errorf("TotalBytes = %d, want 22", item.TotalBytes)
		}

		// Get bytes from offset
		item, err = svc.GetFile("files-test", "range-test.txt", 6, 5)
		if err != nil {
			t.Fatalf("GetFile() with offset error = %v", err)
		}
		if item.Content != "World" {
			t.Errorf("Content = %q, want %q", item.Content, "World")
		}
		if item.Offset != 6 {
			t.Errorf("Offset = %d, want 6", item.Offset)
		}
	})

	t.Run("delete nonexistent file", func(t *testing.T) {
		err := svc.DeleteFile("files-test", "nonexistent.txt")
		if err == nil {
			t.Error("DeleteFile() expected error for nonexistent file")
		}
	})
}

func TestSearch(t *testing.T) {
	svc := createTestService(t)

	// Create playbook with files
	if err := svc.Create("search-test"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, _ = svc.PutFile("search-test", "readme.md", "# Welcome\nThis is a test file", "")
	_, _ = svc.PutFile("search-test", "docs/guide.txt", "User guide content", "")
	_, _ = svc.PutFile("search-test", "other.txt", "Other content here", "")

	t.Run("search by content", func(t *testing.T) {
		items, total, err := svc.Search("search-test", "guide", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 1 {
			t.Errorf("Search() total = %d, want 1", total)
		}
		if len(items) != 1 {
			t.Errorf("Search() returned %d items, want 1", len(items))
		}
	})

	t.Run("search by path", func(t *testing.T) {
		items, total, err := svc.Search("search-test", "readme", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 1 {
			t.Errorf("Search() total = %d, want 1", total)
		}
		_ = items
	})

	t.Run("search case insensitive", func(t *testing.T) {
		items, total, err := svc.Search("search-test", "WELCOME", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 1 {
			t.Errorf("Search() total = %d, want 1", total)
		}
		_ = items
	})

	t.Run("search all playbooks", func(t *testing.T) {
		// Create another playbook
		if err := svc.Create("search-test2"); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		_, _ = svc.PutFile("search-test2", "file.txt", "content with guide word", "")

		items, total, err := svc.Search("", "guide", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 2 {
			t.Errorf("Search() total = %d, want 2", total)
		}
		_ = items
	})

	t.Run("search no matches", func(t *testing.T) {
		items, total, err := svc.Search("search-test", "xyznonexistent", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 0 {
			t.Errorf("Search() total = %d, want 0", total)
		}
		if len(items) != 0 {
			t.Errorf("Search() returned %d items, want 0", len(items))
		}
	})

	t.Run("search empty query", func(t *testing.T) {
		_, _, err := svc.Search("search-test", "", 10, 0)
		if err == nil {
			t.Error("Search() expected error for empty query")
		}
	})

	t.Run("search pagination", func(t *testing.T) {
		// Search for "content" which should match multiple files
		items, total, err := svc.Search("search-test", "content", 1, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(items) != 1 {
			t.Errorf("Search() with limit=1 returned %d items, want 1", len(items))
		}
		if total < 2 {
			t.Errorf("Search() total = %d, want at least 2", total)
		}

		// Get second page
		items2, _, err := svc.Search("search-test", "content", 1, 1)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(items2) != 1 {
			t.Errorf("Search() page 2 returned %d items, want 1", len(items2))
		}
	})
}

func TestListSkipsHiddenAndMeta(t *testing.T) {
	svc := createTestService(t)

	// Create playbook
	if err := svc.Create("hidden-test"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create a regular file
	_, _ = svc.PutFile("hidden-test", "visible.txt", "content", "")

	// Create a file with .meta.json suffix manually (simulating metadata)
	playbookPath := filepath.Join(svc.baseDir, "hidden-test")
	metaFile := filepath.Join(playbookPath, "test.meta.json")
	_ = os.WriteFile(metaFile, []byte("{}"), 0644)

	items, err := svc.ListFiles("hidden-test", "")
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	// Should only have visible.txt (not the .meta.json file)
	if len(items) != 1 {
		t.Errorf("ListFiles() returned %d items, want 1 (should skip .meta.json)", len(items))
	}
	if len(items) > 0 && items[0].Path != "visible.txt" {
		t.Errorf("Expected visible.txt, got %s", items[0].Path)
	}
}
