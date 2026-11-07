//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package tasks

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/projects"
)

// TestFileModes_TaskSetFile: the tasks directory, the task set JSON file and
// its lock file are created with the modes from config.WithFileModes.
func TestFileModes_TaskSetFile(t *testing.T) {
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
	projectsSvc := projects.NewService(cfg, lg)
	svc := NewService(cfg, projectsSvc, lg)

	if _, err := projectsSvc.Create("alice", "Alice", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := svc.CreateTaskSet("alice", "review", "Review", "", nil, false, global.Limits{}, true, ""); err != nil {
		t.Fatalf("CreateTaskSet: %v", err)
	}

	tasksDir := projectsSvc.GetTasksDir("alice")
	testutil.AssertPerm(t, tasksDir, dirMode)
	testutil.AssertPerm(t, filepath.Join(tasksDir, "review.json"), fileMode)
	testutil.AssertPerm(t, filepath.Join(tasksDir, "review.json.lock"), fileMode)
}
