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
	"syscall"
	"unicode/utf8"
)

// AtomicWrite writes content to a file atomically using a temporary file and rename.
// This ensures the file is never left in a partial state.
// Creates missing parent directories with exactly dirMode; the file ends with
// exactly fileMode, whatever the umask (the content is written to a new
// temporary file that replaces it). Callers pass resolved modes (e.g. from
// config.DirMode/FileMode); zero is not replaced by a default.
func AtomicWrite(filePath string, content []byte, dirMode, fileMode fs.FileMode) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := MkdirAllExact(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to a fresh temporary file, created with exactly fileMode before it
	// is renamed into place (a stale one would keep its own mode).
	tempPath := filePath + ".tmp"
	if err := os.Remove(tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove stale temp file: %w", err)
	}
	if err := WriteFileExact(tempPath, content, fileMode); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, filePath); err != nil {
		_ = os.Remove(tempPath) // Clean up on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// MkdirAllExact creates path and any missing parents like os.MkdirAll, giving
// each folder it creates exactly mode whatever the umask. Folders that already
// exist keep their mode.
func MkdirAllExact(path string, mode fs.FileMode) error {
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}
	if parent := filepath.Dir(path); parent != path {
		if err := MkdirAllExact(parent, mode); err != nil {
			return err
		}
	}
	if err := os.Mkdir(path, mode.Perm()); err != nil {
		if os.IsExist(err) {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				return nil // created concurrently: not ours to change
			}
		}
		return err
	}
	return os.Chmod(path, mode.Perm())
}

// OpenFileExact opens path like os.OpenFile. When flag has O_CREATE and this
// call creates the file, the file is set to exactly mode whatever the umask,
// on the open descriptor (never by path). An existing file keeps its mode: it
// may belong to another user who shares the folder.
func OpenFileExact(path string, flag int, mode fs.FileMode) (*os.File, error) {
	if flag&os.O_CREATE == 0 {
		return os.OpenFile(path, flag, mode.Perm())
	}
	f, err := os.OpenFile(path, flag|os.O_EXCL, mode.Perm())
	if err != nil {
		if flag&os.O_EXCL != 0 || !os.IsExist(err) {
			return nil, err
		}
		return os.OpenFile(path, flag&^os.O_CREATE, mode.Perm())
	}
	if err := f.Chmod(mode.Perm()); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

// WriteFileExact writes data to path like os.WriteFile; a file it creates gets
// exactly mode whatever the umask, an existing one keeps its mode (see
// OpenFileExact). The parent folder must exist.
func WriteFileExact(path string, data []byte, mode fs.FileMode) error {
	f, err := OpenFileExact(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
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

// EnsureDir creates a directory, and any missing parents, with exactly mode if
// it doesn't exist. Callers pass a resolved mode (e.g. from config.DirMode);
// zero is not replaced by a default.
func EnsureDir(path string, mode fs.FileMode) error {
	return MkdirAllExact(path, mode)
}
