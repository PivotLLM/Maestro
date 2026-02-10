/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/config"
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

func createTestServiceWithConfig(t *testing.T) (*Service, string) {
	tmpDir, err := os.MkdirTemp("", "projects-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	// Create config file with minimal LLM config
	configPath := filepath.Join(tmpDir, "config.json")
	configContent := `{
		"version": 1,
		"base_dir": "` + tmpDir + `",
		"llms": [
			{
				"id": "test-llm",
				"display_name": "Test LLM",
				"type": "command",
				"command": "/bin/echo",
				"args": ["{{PROMPT}}"],
				"enabled": false,
				"description": "Test LLM"
			}
		]
	}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Load config using New + Load pattern
	cfg := config.New(config.WithConfigPath(configPath))
	if err := cfg.Load(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	logger := createTestLogger(t)

	return NewService(cfg, logger), tmpDir
}

func TestProjectFileOperations(t *testing.T) {
	svc, _ := createTestServiceWithConfig(t)

	// Create a project first
	proj, err := svc.Create("file-test", "Test Project", "For testing files", "", "", "none")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if proj == nil {
		t.Fatal("Create() returned nil project")
	}

	t.Run("put and get file", func(t *testing.T) {
		created, err := svc.PutFile("file-test", "test.txt", "Hello World", "Test file")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}
		if !created {
			t.Error("PutFile() should return created=true for new file")
		}

		item, err := svc.GetFile("file-test", "test.txt", 0, 0)
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
		created, err := svc.PutFile("file-test", "test.txt", "Updated content", "Updated summary")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}
		if created {
			t.Error("PutFile() should return created=false for existing file")
		}

		item, err := svc.GetFile("file-test", "test.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Updated content" {
			t.Errorf("Content = %q, want %q", item.Content, "Updated content")
		}
	})

	t.Run("nested file", func(t *testing.T) {
		_, err := svc.PutFile("file-test", "subdir/nested.txt", "Nested content", "")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}

		item, err := svc.GetFile("file-test", "subdir/nested.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Nested content" {
			t.Errorf("Content = %q, want %q", item.Content, "Nested content")
		}
	})

	t.Run("list files", func(t *testing.T) {
		items, err := svc.ListFiles("file-test", "")
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(items) != 2 {
			t.Errorf("ListFiles() returned %d items, want 2", len(items))
		}
	})

	t.Run("list files with prefix", func(t *testing.T) {
		items, err := svc.ListFiles("file-test", "subdir")
		if err != nil {
			t.Fatalf("ListFiles() error = %v", err)
		}
		if len(items) != 1 {
			t.Errorf("ListFiles() with prefix returned %d items, want 1", len(items))
		}
	})

	t.Run("rename file", func(t *testing.T) {
		err := svc.RenameFile("file-test", "test.txt", "renamed.txt")
		if err != nil {
			t.Fatalf("RenameFile() error = %v", err)
		}

		// Old file should not exist
		_, err = svc.GetFile("file-test", "test.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for renamed file's old path")
		}

		// New file should exist
		item, err := svc.GetFile("file-test", "renamed.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Updated content" {
			t.Errorf("Content = %q, want %q", item.Content, "Updated content")
		}
	})

	t.Run("delete file", func(t *testing.T) {
		err := svc.DeleteFile("file-test", "renamed.txt")
		if err != nil {
			t.Fatalf("DeleteFile() error = %v", err)
		}

		// File should not exist
		_, err = svc.GetFile("file-test", "renamed.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for deleted file")
		}
	})

	t.Run("get nonexistent file", func(t *testing.T) {
		_, err := svc.GetFile("file-test", "nonexistent.txt", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for nonexistent file")
		}
	})

	t.Run("path traversal prevention", func(t *testing.T) {
		_, err := svc.PutFile("file-test", "../escape.txt", "bad content", "")
		if err == nil {
			t.Error("PutFile() expected error for path traversal")
		}

		_, err = svc.GetFile("file-test", "../../etc/passwd", 0, 0)
		if err == nil {
			t.Error("GetFile() expected error for path traversal")
		}
	})

	t.Run("byte range", func(t *testing.T) {
		// Create a file with known content
		_, err := svc.PutFile("file-test", "range-test.txt", "Hello World Byte Range", "")
		if err != nil {
			t.Fatalf("PutFile() error = %v", err)
		}

		// Get first 5 bytes
		item, err := svc.GetFile("file-test", "range-test.txt", 0, 5)
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
		item, err = svc.GetFile("file-test", "range-test.txt", 6, 5)
		if err != nil {
			t.Fatalf("GetFile() with offset error = %v", err)
		}
		if item.Content != "World" {
			t.Errorf("Content = %q, want %q", item.Content, "World")
		}
		if item.Offset != 6 {
			t.Errorf("Offset = %d, want 6", item.Offset)
		}

		// Get full file (no byte range)
		item, err = svc.GetFile("file-test", "range-test.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() full file error = %v", err)
		}
		if item.Content != "Hello World Byte Range" {
			t.Errorf("Content = %q, want %q", item.Content, "Hello World Byte Range")
		}
		if item.TotalBytes != 22 {
			t.Errorf("TotalBytes = %d, want 22", item.TotalBytes)
		}
	})
}

func TestProjectFileSearch(t *testing.T) {
	svc, _ := createTestServiceWithConfig(t)

	// Create project with files
	_, err := svc.Create("search-test", "Search Test", "", "", "", "none")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, _ = svc.PutFile("search-test", "readme.md", "# Welcome\nThis is a test file", "")
	_, _ = svc.PutFile("search-test", "docs/guide.txt", "User guide content", "")
	_, _ = svc.PutFile("search-test", "other.txt", "Other content here", "")

	t.Run("search by content", func(t *testing.T) {
		items, total, err := svc.SearchFiles("search-test", "guide", 10, 0)
		if err != nil {
			t.Fatalf("SearchFiles() error = %v", err)
		}
		if total != 1 {
			t.Errorf("SearchFiles() total = %d, want 1", total)
		}
		if len(items) != 1 {
			t.Errorf("SearchFiles() returned %d items, want 1", len(items))
		}
	})

	t.Run("search by path", func(t *testing.T) {
		items, total, err := svc.SearchFiles("search-test", "readme", 10, 0)
		if err != nil {
			t.Fatalf("SearchFiles() error = %v", err)
		}
		if total != 1 {
			t.Errorf("SearchFiles() total = %d, want 1", total)
		}
		_ = items
	})

	t.Run("search case insensitive", func(t *testing.T) {
		items, total, err := svc.SearchFiles("search-test", "WELCOME", 10, 0)
		if err != nil {
			t.Fatalf("SearchFiles() error = %v", err)
		}
		if total != 1 {
			t.Errorf("SearchFiles() total = %d, want 1", total)
		}
		_ = items
	})

	t.Run("search no matches", func(t *testing.T) {
		items, total, err := svc.SearchFiles("search-test", "xyznonexistent", 10, 0)
		if err != nil {
			t.Fatalf("SearchFiles() error = %v", err)
		}
		if total != 0 {
			t.Errorf("SearchFiles() total = %d, want 0", total)
		}
		if len(items) != 0 {
			t.Errorf("SearchFiles() returned %d items, want 0", len(items))
		}
	})

	t.Run("search empty query", func(t *testing.T) {
		_, _, err := svc.SearchFiles("search-test", "", 10, 0)
		if err == nil {
			t.Error("SearchFiles() expected error for empty query")
		}
	})
}

func TestProjectRename(t *testing.T) {
	svc, _ := createTestServiceWithConfig(t)

	// Create a project
	_, err := svc.Create("original", "Original Project", "", "", "", "none")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add a file to verify it persists after rename
	_, err = svc.PutFile("original", "test.txt", "Test content", "")
	if err != nil {
		t.Fatalf("PutFile() error = %v", err)
	}

	t.Run("rename project", func(t *testing.T) {
		err := svc.Rename("original", "renamed")
		if err != nil {
			t.Fatalf("Rename() error = %v", err)
		}

		// Original should not exist
		if svc.ProjectExists("original") {
			t.Error("Original project should not exist after rename")
		}

		// Renamed should exist
		if !svc.ProjectExists("renamed") {
			t.Error("Renamed project should exist")
		}

		// Files should still be accessible
		item, err := svc.GetFile("renamed", "test.txt", 0, 0)
		if err != nil {
			t.Fatalf("GetFile() error = %v", err)
		}
		if item.Content != "Test content" {
			t.Errorf("Content = %q, want %q", item.Content, "Test content")
		}
	})

	t.Run("rename nonexistent project", func(t *testing.T) {
		err := svc.Rename("nonexistent", "other")
		if err == nil {
			t.Error("Rename() expected error for nonexistent project")
		}
	})

	t.Run("rename to existing name", func(t *testing.T) {
		_, _ = svc.Create("another", "Another", "", "", "", "none")
		err := svc.Rename("another", "renamed")
		if err == nil {
			t.Error("Rename() expected error when destination exists")
		}
	})
}

// NOTE: Subproject file operation tests have been removed during the refactoring.
// Subprojects are no longer supported - use path-based task sets instead.
