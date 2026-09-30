/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package runner

import (
	"context"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/llm"
)

// blockingDispatcher signals when a dispatch starts and holds it until release
// is closed, so a test can observe a run while its goroutine is live.
type blockingDispatcher struct {
	recordingDispatcher
	started chan struct{}
	release chan struct{}
}

func (d *blockingDispatcher) Dispatch(ctx context.Context, req *llm.DispatchRequest) (*llm.DispatchResult, error) {
	select {
	case d.started <- struct{}{}:
	default:
	}
	<-d.release
	return d.recordingDispatcher.Dispatch(ctx, req)
}

// TestRun_ReturnedResultIsNotMutatedByRun: the result Run hands back is the
// state at return time. The background run keeps counting tasks on its own
// copy; a caller marshalling the returned value must never see it change.
func TestRun_ReturnedResultIsNotMutatedByRun(t *testing.T) {
	tr, _, project := newHostDispatchedRunner(t, &recordingDispatcher{})
	d := &blockingDispatcher{started: make(chan struct{}, 1), release: make(chan struct{})}
	tr.llm = d

	if _, err := tr.tasks.CreateTask(project, "main", "worker", "", &global.WorkExecution{Prompt: "do it"}, nil); err != nil {
		t.Fatalf("create task: %v", err)
	}

	res, err := tr.Run(context.Background(), &global.RunRequest{Project: project}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := *res
	if want.Project != project || want.TasksFound != 1 || want.TasksExecuted != 0 || want.Message == "" {
		t.Fatalf("unexpected result at return: %+v", want)
	}

	select {
	case <-d.started:
	case <-time.After(10 * time.Second):
		close(d.release)
		t.Fatal("run goroutine never dispatched")
	}
	if *res != want {
		t.Errorf("result changed while the run was in progress: got %+v, want %+v", *res, want)
	}

	close(d.release)
	tr.Wait()

	if calls, _, _ := d.snapshot(); calls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", calls)
	}
	if *res != want {
		t.Errorf("result changed after the run finished: got %+v, want %+v", *res, want)
	}
}
