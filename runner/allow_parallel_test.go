/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/playbooks"
	"github.com/PivotLLM/Maestro/projects"
	"github.com/PivotLLM/Maestro/reference"
	"github.com/PivotLLM/Maestro/tasks"
)

// newProgrammaticRunner builds a runner the way an embedding host does:
// config.New + Prepare with host options, a host dispatcher, no config file.
func newProgrammaticRunner(t *testing.T, d *recordingDispatcher, opts ...config.Option) (*testRunner, string) {
	t.Helper()
	base := t.TempDir()
	cfg := config.New(append([]config.Option{config.WithBaseDir(base)}, opts...)...)
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	lg, err := logging.New(filepath.Join(base, "test.log"))
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	t.Cleanup(func() { lg.Close() })

	ref := reference.NewService(reference.WithEmbeddedFS(cfg.EmbeddedFS()), reference.WithLogger(lg))
	pb := playbooks.NewService(cfg.PlaybooksDir(), lg)
	pr := projects.NewService(cfg, lg)
	ts := tasks.NewService(cfg, pr, lg)
	r := New(cfg, lg, nil, pb, ref, llm.Dispatcher(d), ts, pr)
	r.SetHostDispatched(true)

	project := "par-project"
	if _, err := pr.Create(project, "Parallel", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	// parallel=true on the task set: the LLM has asked for parallel execution.
	if _, err := ts.CreateTaskSet(project, "main", "Main", "", nil, true, global.Limits{}, true, ""); err != nil {
		t.Fatalf("create task set: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := ts.CreateTask(project, "main", "worker", "", &global.WorkExecution{Prompt: "go"}, nil); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}
	return &testRunner{Runner: r, projects: pr, tasks: ts}, project
}

func projectLog(t *testing.T, tr *testRunner, project string) string {
	t.Helper()
	res, err := tr.projects.GetLog(project, "", 0, 0)
	if err != nil {
		t.Fatalf("get log: %v", err)
	}
	return strings.Join(res.Events, "\n")
}

func falsePtr() *bool { b := false; return &b }

// TestRun_AllowParallelFalse_RunsSequentially: with allow_parallel=false a
// requested parallel run is executed sequentially and the project log says so.
func TestRun_AllowParallelFalse_RunsSequentially(t *testing.T) {
	d := &recordingDispatcher{}
	tr, project := newProgrammaticRunner(t, d, config.WithRunner(config.Runner{AllowParallel: falsePtr()}))

	if _, err := tr.Run(context.Background(), &global.RunRequest{Project: project}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr.Runner.Wait()

	if calls, _, _ := d.snapshot(); calls != 2 {
		t.Errorf("dispatch calls = %d, want 2 (tasks still run)", calls)
	}
	if log := projectLog(t, tr, project); !strings.Contains(log, "not allowed by runner configuration") {
		t.Errorf("project log lacks the sequential-fallback notice:\n%s", log)
	}
}

// TestRun_AllowParallelDefault_Honoured: without the setting a requested
// parallel run is honoured and no fallback notice is logged.
func TestRun_AllowParallelDefault_Honoured(t *testing.T) {
	d := &recordingDispatcher{}
	tr, project := newProgrammaticRunner(t, d)

	if _, err := tr.Run(context.Background(), &global.RunRequest{Project: project}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	tr.Runner.Wait()

	if calls, _, _ := d.snapshot(); calls != 2 {
		t.Errorf("dispatch calls = %d, want 2", calls)
	}
	if log := projectLog(t, tr, project); strings.Contains(log, "not allowed by runner configuration") {
		t.Errorf("unexpected fallback notice when parallel is allowed:\n%s", log)
	}
	_ = os.PathSeparator
}
