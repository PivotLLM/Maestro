/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package reference

import (
	"embed"
	"os"
	"testing"

	"github.com/PivotLLM/Maestro/logging"
)

// Test embedded filesystem - mirrors the real reference directory structure
//
//go:embed testdata/*
var testFS embed.FS

func createTestLogger(t *testing.T) *logging.Logger {
	// Create a temp file for the logger
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

func TestValidatePath(t *testing.T) {
	logger := createTestLogger(t)

	svc := &Service{
		fs:     testFS,
		prefix: "testdata",
		logger: logger,
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "simple path",
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "nested path",
			path:    "subdir/file.txt",
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			path:    "../outside.txt",
			wantErr: true,
		},
		{
			name:    "path traversal nested",
			path:    "subdir/../../outside.txt",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.validatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestList(t *testing.T) {
	logger := createTestLogger(t)

	svc := &Service{
		fs:     testFS,
		prefix: "testdata",
		logger: logger,
	}

	items, err := svc.List("")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have at least the test.txt file
	if len(items) == 0 {
		t.Error("List() returned no items, expected at least 1")
	}

	// Check that test.txt is in the list
	found := false
	for _, item := range items {
		if item.Path == "test.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Error("List() did not return test.txt")
	}
}

func TestGet(t *testing.T) {
	logger := createTestLogger(t)

	svc := &Service{
		fs:     testFS,
		prefix: "testdata",
		logger: logger,
	}

	t.Run("existing file", func(t *testing.T) {
		item, err := svc.Get("test.txt", 0, 0)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if item == nil {
			t.Fatal("Get() returned nil item")
		}
		if item.Path != "test.txt" {
			t.Errorf("Path = %q, want %q", item.Path, "test.txt")
		}
		if item.Content == "" {
			t.Error("Content should not be empty")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := svc.Get("nonexistent.txt", 0, 0)
		if err == nil {
			t.Error("Get() expected error for non-existent file")
		}
	})

	t.Run("path traversal", func(t *testing.T) {
		_, err := svc.Get("../outside.txt", 0, 0)
		if err == nil {
			t.Error("Get() expected error for path traversal")
		}
	})

	t.Run("byte range", func(t *testing.T) {
		// First get full content to know the size
		fullItem, err := svc.Get("test.txt", 0, 0)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		fullContent := fullItem.Content
		totalBytes := fullItem.TotalBytes

		// Get first 5 bytes
		item, err := svc.Get("test.txt", 0, 5)
		if err != nil {
			t.Fatalf("Get() with byte range error = %v", err)
		}
		if len(item.Content) > 5 {
			t.Errorf("Content length = %d, want <= 5", len(item.Content))
		}
		if item.Offset != 0 {
			t.Errorf("Offset = %d, want 0", item.Offset)
		}
		if item.TotalBytes != totalBytes {
			t.Errorf("TotalBytes = %d, want %d", item.TotalBytes, totalBytes)
		}
		if item.Content != fullContent[:5] {
			t.Errorf("Content = %q, want %q", item.Content, fullContent[:5])
		}

		// Get bytes from offset
		item, err = svc.Get("test.txt", 2, 3)
		if err != nil {
			t.Fatalf("Get() with offset error = %v", err)
		}
		if item.Offset != 2 {
			t.Errorf("Offset = %d, want 2", item.Offset)
		}
		if item.Content != fullContent[2:5] {
			t.Errorf("Content = %q, want %q", item.Content, fullContent[2:5])
		}
	})
}

func TestSearch(t *testing.T) {
	logger := createTestLogger(t)

	svc := &Service{
		fs:     testFS,
		prefix: "testdata",
		logger: logger,
	}

	t.Run("search by content", func(t *testing.T) {
		items, total, err := svc.Search("Test", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total == 0 {
			t.Error("Search() expected at least 1 match")
		}
		if len(items) == 0 {
			t.Error("Search() returned no items")
		}
	})

	t.Run("search by path", func(t *testing.T) {
		items, total, err := svc.Search("test", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total == 0 {
			t.Error("Search() expected at least 1 match for path search")
		}
		_ = items
	})

	t.Run("empty query", func(t *testing.T) {
		_, _, err := svc.Search("", 10, 0)
		if err == nil {
			t.Error("Search() expected error for empty query")
		}
	})

	t.Run("no matches", func(t *testing.T) {
		items, total, err := svc.Search("xyznonexistent123", 10, 0)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if total != 0 {
			t.Errorf("Search() total = %d, want 0 for no matches", total)
		}
		if len(items) != 0 {
			t.Errorf("Search() len(items) = %d, want 0", len(items))
		}
	})
}
