/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePathWithinDir validates that a relative path, when resolved against baseDir,
// stays within baseDir. This prevents path traversal attacks.
// Returns the absolute resolved path if valid, or an error if path traversal is detected.
func ValidatePathWithinDir(baseDir, relativePath string) (string, error) {
	// Reject absolute paths - they must be relative
	if filepath.IsAbs(relativePath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", relativePath)
	}

	// Clean the path to normalize . and .. components
	cleanPath := filepath.Clean(relativePath)

	// Get absolute base directory
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base directory: %w", err)
	}

	// Resolve the full path
	absFilePath, err := filepath.Abs(filepath.Join(absBaseDir, cleanPath))
	if err != nil {
		return "", fmt.Errorf("failed to get absolute file path: %w", err)
	}

	// Verify the resolved path is under the base directory
	// We check both that it starts with the base dir + separator, OR equals the base dir exactly
	if !strings.HasPrefix(absFilePath, absBaseDir+string(filepath.Separator)) &&
		absFilePath != absBaseDir {
		return "", fmt.Errorf("path traversal attempt detected: %s", relativePath)
	}

	return absFilePath, nil
}

// CleanRelativePath cleans a relative path by removing . and .. components
// and normalizing path separators.
func CleanRelativePath(path string) string {
	return filepath.Clean(path)
}

// IsPathWithin checks if resolvedPath is within or equal to baseDir.
// Both paths should be absolute.
func IsPathWithin(baseDir, resolvedPath string) bool {
	return strings.HasPrefix(resolvedPath, baseDir+string(filepath.Separator)) ||
		resolvedPath == baseDir
}
