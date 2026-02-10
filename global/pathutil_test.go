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

func TestValidatePathWithinDir(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "pathutil-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmpDir)

	tests := []struct {
		name        string
		baseDir     string
		path        string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid simple path",
			baseDir: tmpDir,
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			baseDir: tmpDir,
			path:    "subdir/file.txt",
			wantErr: false,
		},
		{
			name:    "valid deeply nested path",
			baseDir: tmpDir,
			path:    "a/b/c/d/file.txt",
			wantErr: false,
		},
		{
			name:        "path traversal with ..",
			baseDir:     tmpDir,
			path:        "../outside.txt",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "path traversal nested",
			baseDir:     tmpDir,
			path:        "subdir/../../outside.txt",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:        "path traversal to root",
			baseDir:     tmpDir,
			path:        "../../../../etc/passwd",
			wantErr:     true,
			errContains: "path traversal",
		},
		{
			name:    "path with dot current dir",
			baseDir: tmpDir,
			path:    "./file.txt",
			wantErr: false,
		},
		{
			name:    "path with internal dots",
			baseDir: tmpDir,
			path:    "subdir/./file.txt",
			wantErr: false,
		},
		{
			name:        "absolute path rejected",
			baseDir:     tmpDir,
			path:        "/etc/passwd",
			wantErr:     true,
			errContains: "absolute paths not allowed",
		},
		{
			name:        "absolute path with traversal rejected",
			baseDir:     tmpDir,
			path:        "/var/../etc/passwd",
			wantErr:     true,
			errContains: "absolute paths not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePathWithinDir(tt.baseDir, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePathWithinDir() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("ValidatePathWithinDir() error = %v, want error containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePathWithinDir() unexpected error: %v", err)
				}
				// Verify result is under base dir
				if !IsPathWithin(tmpDir, result) {
					t.Errorf("ValidatePathWithinDir() result %s is not within %s", result, tmpDir)
				}
			}
		})
	}
}

func TestCleanRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "file.txt",
			expected: "file.txt",
		},
		{
			name:     "path with dot",
			path:     "./file.txt",
			expected: "file.txt",
		},
		{
			name:     "nested with dots",
			path:     "a/./b/../c/file.txt",
			expected: "a/c/file.txt",
		},
		{
			name:     "trailing slash",
			path:     "dir/",
			expected: "dir",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanRelativePath(tt.path)
			// Note: filepath.Clean is platform-specific, so we compare using filepath
			expected := filepath.Clean(tt.expected)
			if result != expected {
				t.Errorf("CleanRelativePath(%q) = %q, want %q", tt.path, result, expected)
			}
		})
	}
}

func TestIsPathWithin(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		path     string
		expected bool
	}{
		{
			name:     "path within base dir",
			baseDir:  "/base/dir",
			path:     "/base/dir/file.txt",
			expected: true,
		},
		{
			name:     "path equals base dir",
			baseDir:  "/base/dir",
			path:     "/base/dir",
			expected: true,
		},
		{
			name:     "path outside base dir",
			baseDir:  "/base/dir",
			path:     "/base/other/file.txt",
			expected: false,
		},
		{
			name:     "path is parent of base dir",
			baseDir:  "/base/dir",
			path:     "/base",
			expected: false,
		},
		{
			name:     "similar prefix but different dir",
			baseDir:  "/base/dir",
			path:     "/base/directory/file.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPathWithin(tt.baseDir, tt.path)
			if result != tt.expected {
				t.Errorf("IsPathWithin(%q, %q) = %v, want %v", tt.baseDir, tt.path, result, tt.expected)
			}
		})
	}
}

// contains checks if s contains substr
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
