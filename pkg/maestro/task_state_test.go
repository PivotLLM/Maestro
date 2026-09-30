// Maestro
// License: MIT

package maestro

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/global"
)

const stateProject = "state"
const stateTaskSet = "analysis"

// newStateProvider registers a host-dispatched provider and creates the test
// project and task set.
func newStateProvider(t *testing.T) *Provider {
	t.Helper()
	p := &Provider{}
	p.RegisterTools(toolspec.Deps{Cfg: newPreparedConfig(t), Host: HostDeps{Dispatcher: stubDispatcher{}}})
	if _, err := p.projects.Create(stateProject, "State", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := p.tasks.CreateTaskSet(stateProject, stateTaskSet, "Analysis", "", nil, false, global.Limits{}, false, ""); err != nil {
		t.Fatalf("create task set: %v", err)
	}
	return p
}

// createStateTask creates a task already in the given work and QA state.
func createStateTask(t *testing.T, p *Provider, title string, work global.WorkExecution, qa *global.QAExecution) *global.Task {
	t.Helper()
	work.Prompt = "do it"
	task, err := p.tasks.CreateTask(stateProject, stateTaskSet, title, "", &work, qa)
	if err != nil {
		t.Fatalf("create task %s: %v", title, err)
	}
	return task
}

func getStateTask(t *testing.T, p *Provider, uuid string) *global.Task {
	t.Helper()
	task, _, err := p.tasks.GetTask(stateProject, uuid)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	return task
}

func TestHandleTaskUpdate_WorkStatus(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		wantError  string
		wantStatus string
		wantTitle  string
	}{
		{
			name:       "waiting resets a failed task",
			args:       map[string]any{"work_status": "waiting"},
			wantStatus: global.ExecutionStatusWaiting,
			wantTitle:  "Alice",
		},
		{
			name:       "unknown status is rejected",
			args:       map[string]any{"work_status": "bogus"},
			wantError:  `invalid work_status "bogus": must be one of waiting, processing, retry, failed, error, done`,
			wantStatus: global.ExecutionStatusFailed,
			wantTitle:  "Alice",
		},
		{
			name:       "title only leaves status unchanged",
			args:       map[string]any{"title": "Bob"},
			wantStatus: global.ExecutionStatusFailed,
			wantTitle:  "Bob",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newStateProvider(t)
			task := createStateTask(t, p, "Alice", global.WorkExecution{Status: global.ExecutionStatusFailed, Invocations: 3}, nil)

			tc.args["project"] = stateProject
			tc.args["uuid"] = task.UUID
			res, err := p.handleTaskUpdate(&toolspec.ToolCall{Args: tc.args})
			if err != nil {
				t.Fatalf("handleTaskUpdate: %v", err)
			}

			if tc.wantError != "" {
				if !res.IsError || res.ForLLM != tc.wantError {
					t.Errorf("result = %+v, want error %q", res, tc.wantError)
				}
			} else {
				if res.IsError {
					t.Fatalf("unexpected error result: %s", res.ForLLM)
				}
				var got global.Task
				if err := json.Unmarshal([]byte(res.ForLLM), &got); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if got.Work.Status != tc.wantStatus || got.Title != tc.wantTitle {
					t.Errorf("response status=%q title=%q, want %q %q", got.Work.Status, got.Title, tc.wantStatus, tc.wantTitle)
				}
			}

			stored := getStateTask(t, p, task.UUID)
			if stored.Work.Status != tc.wantStatus || stored.Title != tc.wantTitle {
				t.Errorf("stored status=%q title=%q, want %q %q", stored.Work.Status, stored.Title, tc.wantStatus, tc.wantTitle)
			}
		})
	}
}

func TestHandleTaskSetReset_Modes(t *testing.T) {
	doneQA := func(verdict string) *global.QAExecution {
		return &global.QAExecution{Enabled: true, Status: global.ExecutionStatusDone, Verdict: verdict, Invocations: 1}
	}
	done := global.WorkExecution{Status: global.ExecutionStatusDone, Invocations: 1}

	tests := []struct {
		name      string
		mode      string
		wantReset string // title of the one task expected to be reset
		wantMsg   string
	}{
		{name: "escalated resets only the escalated task", mode: "escalated", wantReset: "escalated", wantMsg: "Reset 1 escalated tasks to waiting status."},
		{name: "failed resets only the failed task", mode: "failed", wantReset: "failed", wantMsg: "Reset 1 failed tasks to waiting status."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := newStateProvider(t)
			byTitle := map[string]*global.Task{
				"passed":    createStateTask(t, p, "passed", done, doneQA(global.QAVerdictPass)),
				"failed":    createStateTask(t, p, "failed", global.WorkExecution{Status: global.ExecutionStatusFailed, Invocations: 2, Error: "boom"}, &global.QAExecution{Enabled: true}),
				"escalated": createStateTask(t, p, "escalated", done, doneQA(global.QAVerdictEscalate)),
				// A stale escalate verdict on a task whose QA is off is not an escalated task.
				"qa-off": createStateTask(t, p, "qa-off", done, &global.QAExecution{Enabled: false, Verdict: global.QAVerdictEscalate}),
			}
			resultsDir := p.tasks.GetResultsDir(stateProject)
			if err := os.MkdirAll(resultsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, task := range byTitle {
				if err := os.WriteFile(filepath.Join(resultsDir, task.UUID+".json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			before := map[string]global.Task{}
			for title, task := range byTitle {
				before[title] = *getStateTask(t, p, task.UUID)
			}

			res, err := p.handleTaskSetReset(&toolspec.ToolCall{Args: map[string]any{"project": stateProject, "path": stateTaskSet, "mode": tc.mode}})
			if err != nil {
				t.Fatalf("handleTaskSetReset: %v", err)
			}
			if res.IsError {
				t.Fatalf("unexpected error result: %s", res.ForLLM)
			}
			var out struct {
				TasksReset int    `json:"tasks_reset"`
				Message    string `json:"message"`
			}
			if err := json.Unmarshal([]byte(res.ForLLM), &out); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if out.TasksReset != 1 || out.Message != tc.wantMsg {
				t.Errorf("tasks_reset=%d message=%q, want 1 %q", out.TasksReset, out.Message, tc.wantMsg)
			}

			for title, task := range byTitle {
				got := getStateTask(t, p, task.UUID)
				_, statErr := os.Stat(filepath.Join(resultsDir, task.UUID+".json"))
				if title != tc.wantReset {
					if got.Work != before[title].Work || got.QA != before[title].QA {
						t.Errorf("%s changed: work=%+v qa=%+v", title, got.Work, got.QA)
					}
					if statErr != nil {
						t.Errorf("%s result file removed: %v", title, statErr)
					}
					continue
				}
				wantWork := before[title].Work
				wantWork.Status, wantWork.Invocations, wantWork.Error, wantWork.LastAttemptAt = global.ExecutionStatusWaiting, 0, "", nil
				wantQA := before[title].QA
				wantQA.Status, wantQA.Invocations, wantQA.Error, wantQA.Verdict = global.ExecutionStatusWaiting, 0, "", ""
				if got.Work != wantWork || got.QA != wantQA {
					t.Errorf("%s after reset: work=%+v qa=%+v, want work=%+v qa=%+v", title, got.Work, got.QA, wantWork, wantQA)
				}
				if !os.IsNotExist(statErr) {
					t.Errorf("%s result file not removed: %v", title, statErr)
				}
			}
		})
	}
}

func TestHandleTaskSetReset_UnknownMode(t *testing.T) {
	p := newStateProvider(t)
	task := createStateTask(t, p, "Alice", global.WorkExecution{Status: global.ExecutionStatusFailed}, nil)

	res, err := p.handleTaskSetReset(&toolspec.ToolCall{Args: map[string]any{"project": stateProject, "path": stateTaskSet, "mode": "bogus"}})
	if err != nil {
		t.Fatalf("handleTaskSetReset: %v", err)
	}
	if !res.IsError {
		t.Fatalf("unknown mode accepted: %s", res.ForLLM)
	}
	for _, mode := range []string{"'all'", "'failed'", "'escalated'"} {
		if !strings.Contains(res.ForLLM, mode) {
			t.Errorf("error %q does not list %s", res.ForLLM, mode)
		}
	}
	if got := getStateTask(t, p, task.UUID); got.Work.Status != global.ExecutionStatusFailed {
		t.Errorf("status after rejected reset = %q, want failed", got.Work.Status)
	}
}
