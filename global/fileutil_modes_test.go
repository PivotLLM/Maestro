//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

const (
	testDirMode  fs.FileMode = 0o770
	testFileMode fs.FileMode = 0o660
)

// TestMkdirAllExact: every folder the call creates gets exactly the mode under
// a narrowing umask; a folder that already existed keeps its own mode.
func TestMkdirAllExact(t *testing.T) {
	testutil.NarrowUmask(t)
	root := t.TempDir()
	existing := filepath.Join(root, "alice")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(existing, "a", "b")
	if err := global.MkdirAllExact(target, testDirMode); err != nil {
		t.Fatalf("MkdirAllExact: %v", err)
	}
	testutil.AssertPerm(t, existing, 0o700)
	testutil.AssertPerm(t, filepath.Join(existing, "a"), testDirMode)
	testutil.AssertPerm(t, target, testDirMode)

	// Again on an existing tree: no error, nothing changed.
	if err := global.MkdirAllExact(target, 0o750); err != nil {
		t.Fatalf("MkdirAllExact existing: %v", err)
	}
	testutil.AssertPerm(t, target, testDirMode)
}

// TestMkdirAllExact_FileInPath: a file where a folder is wanted is an error.
func TestMkdirAllExact_FileInPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bob")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := global.MkdirAllExact(file, testDirMode); err == nil {
		t.Error("MkdirAllExact on a file: want error")
	}
	if err := global.MkdirAllExact(filepath.Join(file, "sub"), testDirMode); err == nil {
		t.Error("MkdirAllExact below a file: want error")
	}
}

// TestEnsureDir_ExactMode: EnsureDir creates missing folders with exactly mode.
func TestEnsureDir_ExactMode(t *testing.T) {
	testutil.NarrowUmask(t)
	dir := filepath.Join(t.TempDir(), "alice", "bob")
	if err := global.EnsureDir(dir, testDirMode); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	testutil.AssertPerm(t, filepath.Dir(dir), testDirMode)
	testutil.AssertPerm(t, dir, testDirMode)
}

// TestOpenFileExact: a file the call creates gets exactly mode; an existing
// file keeps its mode; without O_CREATE a missing file is an error.
func TestOpenFileExact(t *testing.T) {
	testutil.NarrowUmask(t)
	dir := t.TempDir()

	created := filepath.Join(dir, "alice.log")
	f, err := global.OpenFileExact(created, os.O_CREATE|os.O_APPEND|os.O_WRONLY, testFileMode)
	if err != nil {
		t.Fatalf("OpenFileExact create: %v", err)
	}
	if _, err := f.WriteString("one\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	testutil.AssertPerm(t, created, testFileMode)

	// Reopen to append: content kept, mode unchanged.
	f, err = global.OpenFileExact(created, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("OpenFileExact existing: %v", err)
	}
	if _, err := f.WriteString("two\n"); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	testutil.AssertPerm(t, created, testFileMode)
	if data, _ := os.ReadFile(created); string(data) != "one\ntwo\n" {
		t.Errorf("content = %q, want appended lines", data)
	}

	existing := filepath.Join(dir, "bob.txt")
	if err := os.WriteFile(existing, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err = global.OpenFileExact(existing, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, testFileMode)
	if err != nil {
		t.Fatalf("OpenFileExact existing file: %v", err)
	}
	_ = f.Close()
	testutil.AssertPerm(t, existing, 0o600)

	if _, err := global.OpenFileExact(filepath.Join(dir, "missing"), os.O_WRONLY, testFileMode); !os.IsNotExist(err) {
		t.Errorf("OpenFileExact without O_CREATE on missing file: err = %v, want not exist", err)
	}
	if _, err := global.OpenFileExact(existing, os.O_CREATE|os.O_EXCL|os.O_WRONLY, testFileMode); !os.IsExist(err) {
		t.Errorf("OpenFileExact with O_EXCL on existing file: err = %v, want exist", err)
	}
}

// TestWriteFileExact: a new file gets exactly mode and the content.
func TestWriteFileExact(t *testing.T) {
	testutil.NarrowUmask(t)
	path := filepath.Join(t.TempDir(), "alice.json")
	if err := global.WriteFileExact(path, []byte("{}"), testFileMode); err != nil {
		t.Fatalf("WriteFileExact: %v", err)
	}
	testutil.AssertPerm(t, path, testFileMode)
	if data, _ := os.ReadFile(path); string(data) != "{}" {
		t.Errorf("content = %q, want {}", data)
	}
}

// TestAtomicWrite_ExactModes: the parent folders and the file get exactly the
// modes under a narrowing umask, also when replacing an existing file of
// another mode and when a stale temp file of another mode is left behind.
func TestAtomicWrite_ExactModes(t *testing.T) {
	testutil.NarrowUmask(t)
	root := t.TempDir()

	path := filepath.Join(root, "alice", "bob", "notes.md")
	if err := global.AtomicWrite(path, []byte("one"), testDirMode, testFileMode); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}
	testutil.AssertPerm(t, filepath.Join(root, "alice"), testDirMode)
	testutil.AssertPerm(t, filepath.Join(root, "alice", "bob"), testDirMode)
	testutil.AssertPerm(t, path, testFileMode)

	replaced := filepath.Join(root, "replaced.md")
	if err := os.WriteFile(replaced, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replaced+".tmp", []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := global.AtomicWrite(replaced, []byte("new"), testDirMode, testFileMode); err != nil {
		t.Fatalf("AtomicWrite replace: %v", err)
	}
	testutil.AssertPerm(t, replaced, testFileMode)
	if data, _ := os.ReadFile(replaced); string(data) != "new" {
		t.Errorf("content = %q, want new", data)
	}
	if _, err := os.Stat(replaced + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file left behind: %v", err)
	}
}
