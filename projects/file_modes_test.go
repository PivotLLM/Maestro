//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
	"github.com/PivotLLM/Maestro/logging"
)

// TestFileModes_ReachCreatedFiles: the modes from config.WithFileModes (or
// exactly the defaults without it) reach the directories and files a project
// creates: the logger's file, project.json, the project and task logs, and
// atomically written project files with their metadata.
func TestFileModes_ReachCreatedFiles(t *testing.T) {
	testutil.ZeroUmask(t)
	tests := []struct {
		name              string
		opts              []config.Option
		wantDir, wantFile fs.FileMode
	}{
		{"configured", []config.Option{config.WithFileModes(0o750, 0o640)}, 0o750, 0o640},
		{"default", nil, 0o755, 0o644},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.New(append([]config.Option{config.WithBaseDir(t.TempDir())}, tt.opts...)...)
			if err := cfg.Prepare(); err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			logPath := filepath.Join(t.TempDir(), "logs", "test.log")
			lg, err := logging.New(logPath, cfg.DirMode(), cfg.FileMode())
			if err != nil {
				t.Fatalf("logger: %v", err)
			}
			t.Cleanup(func() { _ = lg.Close() })
			svc := NewService(cfg, lg)

			if _, err := svc.Create("alice", "Alice", "", "", "", "none"); err != nil {
				t.Fatalf("Create: %v", err)
			}
			if err := svc.AppendLog("alice", "", "project entry"); err != nil {
				t.Fatalf("AppendLog project: %v", err)
			}
			if err := svc.AppendLog("alice", "t1", "task entry"); err != nil {
				t.Fatalf("AppendLog task: %v", err)
			}
			if _, err := svc.PutFile("alice", "notes/bob.md", "hello", "summary"); err != nil {
				t.Fatalf("PutFile: %v", err)
			}

			projectDir := filepath.Join(cfg.ProjectsDir(), "alice")
			filesDir := filepath.Join(projectDir, "files")
			for _, dir := range []string{
				filepath.Dir(logPath),
				projectDir,
				filepath.Join(projectDir, "results"),
				filesDir,
				filepath.Join(filesDir, "notes"),
			} {
				testutil.AssertPerm(t, dir, tt.wantDir)
			}
			for _, file := range []string{
				logPath,
				filepath.Join(projectDir, global.ProjectFileName),
				filepath.Join(projectDir, global.ProjectLogName),
				filepath.Join(projectDir, "results", "task-t1.log"),
				filepath.Join(filesDir, "notes", "bob.md"),
				filepath.Join(filesDir, "notes", "bob.md"+global.MetaSuffix),
			} {
				testutil.AssertPerm(t, file, tt.wantFile)
			}
		})
	}
}

// TestFileModes_ImportUsesConfiguredMode: imported files get the configured
// file mode, not the source file's mode, for both a single file and a
// directory import.
func TestFileModes_ImportUsesConfiguredMode(t *testing.T) {
	testutil.ZeroUmask(t)
	const dirMode, fileMode fs.FileMode = 0o750, 0o640

	cfg := config.New(config.WithBaseDir(t.TempDir()), config.WithFileModes(dirMode, fileMode))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	lg, err := logging.New(filepath.Join(t.TempDir(), "test.log"), cfg.DirMode(), cfg.FileMode())
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	t.Cleanup(func() { _ = lg.Close() })
	svc := NewService(cfg, lg)
	if _, err := svc.Create("alice", "Alice", "", "", "", "none"); err != nil {
		t.Fatalf("Create: %v", err)
	}

	srcRoot := t.TempDir()
	single := filepath.Join(srcRoot, "single.txt")
	if err := os.WriteFile(single, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	srcDir := filepath.Join(srcRoot, "bob")
	if err := os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "sub", "nested.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}

	singleRes, err := svc.ImportFiles("alice", single, false)
	if err != nil || singleRes.FilesImported != 1 {
		t.Fatalf("import single: res=%+v err=%v", singleRes, err)
	}
	dirRes, err := svc.ImportFiles("alice", srcDir, true)
	if err != nil || dirRes.FilesImported != 1 {
		t.Fatalf("import dir: res=%+v err=%v", dirRes, err)
	}

	filesDir := filepath.Join(cfg.ProjectsDir(), "alice", "files")
	testutil.AssertPerm(t, filepath.Join(filesDir, singleRes.ImportedTo), fileMode)
	testutil.AssertPerm(t, filepath.Join(filesDir, dirRes.ImportedTo, "sub"), dirMode)
	testutil.AssertPerm(t, filepath.Join(filesDir, dirRes.ImportedTo, "sub", "nested.txt"), fileMode)
}
