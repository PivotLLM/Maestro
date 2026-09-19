/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/logging"
)

func newImportTestService(t *testing.T) *Service {
	t.Helper()
	cfg := config.New(config.WithBaseDir(t.TempDir()))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	lg, err := logging.New(filepath.Join(t.TempDir(), "test.log"))
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	t.Cleanup(func() { lg.Close() })
	svc := NewService(cfg, lg)
	if _, err := svc.Create("imp", "Import", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	return svc
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestImportFiles_NoPredicate_AnyPath: standalone behaviour is unchanged —
// without a predicate any readable path can be imported.
func TestImportFiles_NoPredicate_AnyPath(t *testing.T) {
	svc := newImportTestService(t)
	src := filepath.Join(t.TempDir(), "anywhere.txt")
	writeFile(t, src, "x")
	res, err := svc.ImportFiles("imp", src, false)
	if err != nil || res.FilesImported != 1 {
		t.Fatalf("import without predicate: res=%+v err=%v", res, err)
	}
}

// TestImportFiles_PredicateRefusesSource: a top-level source outside the
// permitted area is refused outright, naming the path.
func TestImportFiles_PredicateRefusesSource(t *testing.T) {
	svc := newImportTestService(t)
	allowed := t.TempDir()
	svc.SetImportAllowed(func(p string) bool { return strings.HasPrefix(p, allowed+string(os.PathSeparator)) })

	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "x")
	if _, err := svc.ImportFiles("imp", outside, false); err == nil || !strings.Contains(err.Error(), "import refused") || !strings.Contains(err.Error(), outside) {
		t.Fatalf("outside import: err = %v, want refusal naming the path", err)
	}

	inside := filepath.Join(allowed, "ok.txt")
	writeFile(t, inside, "y")
	res, err := svc.ImportFiles("imp", inside, false)
	if err != nil || res.FilesImported != 1 {
		t.Fatalf("inside import: res=%+v err=%v", res, err)
	}
}

// TestImportFiles_PredicateSkipsEntriesInsideDirectory: within a permitted
// directory, a symlink pointing outside the permitted area is skipped and
// counted rather than copied.
func TestImportFiles_PredicateSkipsEntriesInsideDirectory(t *testing.T) {
	svc := newImportTestService(t)
	allowed := t.TempDir()
	svc.SetImportAllowed(func(p string) bool { return strings.HasPrefix(p, allowed+string(os.PathSeparator)) })

	dir := filepath.Join(allowed, "docs")
	writeFile(t, filepath.Join(dir, "a.md"), "a")
	writeFile(t, filepath.Join(dir, "sub", "b.md"), "b")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, outside, "s")
	if err := os.Symlink(outside, filepath.Join(dir, "leak")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	res, err := svc.ImportFiles("imp", dir, true)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.FilesImported != 2 || res.FilesRefused != 1 || res.LinksImported != 0 {
		t.Errorf("result = %+v, want 2 files imported, 1 refused, 0 links", res)
	}
	if _, err := os.Lstat(filepath.Join(svc.getFilesDir("imp"), "imported", "docs", "leak")); !os.IsNotExist(err) {
		t.Error("refused symlink was copied into the project")
	}
}
