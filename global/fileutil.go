/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"unicode/utf8"
)

// AtomicWrite writes content to a file atomically using a temporary file and rename.
// This ensures the file is never left in a partial state.
// Creates parent directories with dirMode if they don't exist; the file is
// created with fileMode. Callers pass resolved modes (e.g. from
// config.DirMode/FileMode); zero is not replaced by a default.
func AtomicWrite(filePath string, content []byte, dirMode, fileMode fs.FileMode) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temporary file
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, content, fileMode); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// IsValidUTF8File checks if a file contains valid UTF-8 text.
// Returns an error if the file cannot be read or contains invalid UTF-8/binary data.
func IsValidUTF8File(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	if !utf8.Valid(data) {
		return fmt.Errorf("file contains invalid UTF-8 or appears to be binary")
	}

	return nil
}

// IsValidUTF8 checks if byte slice contains valid UTF-8 text.
func IsValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil && info.IsDir()
}

// EnsureDir creates a directory with mode if it doesn't exist. Callers pass a
// resolved mode (e.g. from config.DirMode); zero is not replaced by a default.
func EnsureDir(path string, mode fs.FileMode) error {
	return os.MkdirAll(path, mode)
}
