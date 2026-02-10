/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "fileutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("write new file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "new-file.txt")
		content := []byte("Hello, World!")

		err := AtomicWrite(filePath, content)
		if err != nil {
			t.Fatalf("AtomicWrite() error = %v", err)
		}

		// Verify file exists and has correct content
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("File content = %q, want %q", string(data), string(content))
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "existing-file.txt")

		// Create initial file
		if err := os.WriteFile(filePath, []byte("old content"), 0644); err != nil {
			t.Fatalf("Failed to create initial file: %v", err)
		}

		newContent := []byte("new content")
		err := AtomicWrite(filePath, newContent)
		if err != nil {
			t.Fatalf("AtomicWrite() error = %v", err)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(data) != string(newContent) {
			t.Errorf("File content = %q, want %q", string(data), string(newContent))
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "a", "b", "c", "nested-file.txt")
		content := []byte("nested content")

		err := AtomicWrite(filePath, content)
		if err != nil {
			t.Fatalf("AtomicWrite() error = %v", err)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("File content = %q, want %q", string(data), string(content))
		}
	})

	t.Run("no temp file left on success", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "clean-file.txt")
		tempPath := filePath + ".tmp"

		err := AtomicWrite(filePath, []byte("content"))
		if err != nil {
			t.Fatalf("AtomicWrite() error = %v", err)
		}

		// Verify no temp file exists
		if FileExists(tempPath) {
			t.Error("Temp file should not exist after successful write")
		}
	})
}

func TestIsValidUTF8File(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("valid UTF-8 file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "valid-utf8.txt")
		content := []byte("Hello, 世界! 🎉")
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		err := IsValidUTF8File(filePath)
		if err != nil {
			t.Errorf("IsValidUTF8File() error = %v, want nil", err)
		}
	})

	t.Run("invalid UTF-8 file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "invalid-utf8.txt")
		// Invalid UTF-8 sequence
		content := []byte{0xFF, 0xFE, 0x00, 0x01}
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		err := IsValidUTF8File(filePath)
		if err == nil {
			t.Error("IsValidUTF8File() expected error for invalid UTF-8, got nil")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "non-existent.txt")

		err := IsValidUTF8File(filePath)
		if err == nil {
			t.Error("IsValidUTF8File() expected error for non-existent file, got nil")
		}
	})
}

func TestIsValidUTF8(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "valid ASCII",
			data:     []byte("Hello, World!"),
			expected: true,
		},
		{
			name:     "valid UTF-8 with multibyte",
			data:     []byte("Hello, 世界!"),
			expected: true,
		},
		{
			name:     "valid UTF-8 with emoji",
			data:     []byte("🎉🎊🎈"),
			expected: true,
		},
		{
			name:     "empty data",
			data:     []byte{},
			expected: true,
		},
		{
			name:     "invalid UTF-8",
			data:     []byte{0xFF, 0xFE},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidUTF8(tt.data)
			if result != tt.expected {
				t.Errorf("IsValidUTF8(%v) = %v, want %v", tt.data, result, tt.expected)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("existing file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "exists.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		if !FileExists(filePath) {
			t.Error("FileExists() = false, want true for existing file")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "not-exists.txt")

		if FileExists(filePath) {
			t.Error("FileExists() = true, want false for non-existent file")
		}
	})

	t.Run("directory is not a file", func(t *testing.T) {
		if FileExists(tmpDir) {
			t.Error("FileExists() = true, want false for directory")
		}
	})
}

func TestDirExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("existing directory", func(t *testing.T) {
		if !DirExists(tmpDir) {
			t.Error("DirExists() = false, want true for existing directory")
		}
	})

	t.Run("non-existent directory", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "not-exists")

		if DirExists(dirPath) {
			t.Error("DirExists() = true, want false for non-existent directory")
		}
	})

	t.Run("file is not a directory", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "file.txt")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}

		if DirExists(filePath) {
			t.Error("DirExists() = true, want false for file")
		}
	})
}

func TestEnsureDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	t.Run("create new directory", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "new-dir")

		err := EnsureDir(dirPath)
		if err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}

		if !DirExists(dirPath) {
			t.Error("Directory was not created")
		}
	})

	t.Run("create nested directories", func(t *testing.T) {
		dirPath := filepath.Join(tmpDir, "a", "b", "c")

		err := EnsureDir(dirPath)
		if err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}

		if !DirExists(dirPath) {
			t.Error("Nested directories were not created")
		}
	})

	t.Run("existing directory is fine", func(t *testing.T) {
		err := EnsureDir(tmpDir)
		if err != nil {
			t.Errorf("EnsureDir() error = %v for existing directory", err)
		}
	})
}
