// Maestro
// License: MIT

package maestro

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/runner"
	"github.com/PivotLLM/toolspec"
)

func TestCompletionSink_NilWhenNoNotify(t *testing.T) {
	if completionSink(nil) != nil {
		t.Error("completionSink(nil) should be nil")
	}
	if completionSink(&toolspec.ToolCall{Notify: nil}) != nil {
		t.Error("completionSink with nil Notify should be nil")
	}
}

func TestCompletionSink_DeliversParsedPayload(t *testing.T) {
	var got *toolspec.Result
	call := &toolspec.ToolCall{Notify: func(r *toolspec.Result) { got = r }}
	sink := completionSink(call)
	if sink == nil {
		t.Fatal("expected a non-nil sink when Notify is set")
	}

	payload, _ := json.Marshal(runner.CallbackPayload{
		Event:   "completed",
		Project: "proj",
		Path:    "taskset/a",
		Tasks: []runner.CallbackTask{
			{ID: 1, Title: "T1", Status: "done", RetrievalInstruction: "call task_result_get ..."},
		},
	})
	sink(payload)

	if got == nil {
		t.Fatal("Notify was not called")
	}
	if got.IsError {
		t.Error("completed event should not be an error")
	}
	if !strings.Contains(got.ForUser, "completed") {
		t.Errorf("ForUser missing completion text: %q", got.ForUser)
	}
	if !strings.Contains(got.ForLLM, "task_result_get") {
		t.Errorf("ForLLM missing retrieval instruction: %q", got.ForLLM)
	}
}

func TestNotificationResult_Failed(t *testing.T) {
	res := notificationResult(&runner.CallbackPayload{
		Event:        "failed",
		Project:      "proj",
		Path:         "taskset/b",
		ErrorCode:    "no_llm_enabled",
		ErrorMessage: "no LLMs are enabled",
		Tasks:        []runner.CallbackTask{{ID: 1, Title: "T1", Status: "failed", Error: "boom"}},
	})
	if !res.IsError {
		t.Error("failed event should set IsError")
	}
	if !strings.Contains(res.ForLLM, "no LLMs are enabled") {
		t.Errorf("ForLLM missing failure message: %q", res.ForLLM)
	}
}

// TestHostDispatch_StripsCallbackURLParam verifies that under host-dispatch the
// callback_url parameter is removed from the exposed tool schemas (the host
// delivers completions via the injected sink instead).
func TestHostDispatch_StripsCallbackURLParam(t *testing.T) {
	defs := []toolspec.ToolDefinition{
		{Name: "task_dispatch", Parameters: []toolspec.Parameter{
			{Name: "project"}, {Name: "callback_url"},
		}},
	}
	out := withoutParam(defs, "callback_url")
	for _, p := range out[0].Parameters {
		if p.Name == "callback_url" {
			t.Fatal("callback_url parameter was not stripped")
		}
	}
	if len(out[0].Parameters) != 1 {
		t.Errorf("expected 1 remaining param, got %d", len(out[0].Parameters))
	}
}
