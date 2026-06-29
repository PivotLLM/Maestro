/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package maestro

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/PivotLLM/Maestro/runner"
	"github.com/PivotLLM/toolspec"
)

// completionSink adapts a per-call Notify hook into a runner.CompletionSink.
// When a taskset run finishes, the runner emits a JSON CallbackPayload; this
// parses it and delivers an agent-facing notification through the host's async
// Notify path — the same mechanism a host uses for a local sub-agent spawn.
// Returns nil when the host provides no async delivery (e.g. standalone
// Maestro), so the runner skips the callback entirely.
func completionSink(call *toolspec.ToolCall) runner.CompletionSink {
	if call == nil || call.Notify == nil {
		return nil
	}
	notify := call.Notify
	return func(payload []byte) {
		var cp runner.CallbackPayload
		if err := json.Unmarshal(payload, &cp); err != nil {
			notify(&toolspec.Result{
				ForLLM: "[TASK NOTIFICATION] A Maestro task run finished, but its completion payload could not be parsed.",
			})
			return
		}
		notify(notificationResult(&cp))
	}
}

// notificationResult renders a CallbackPayload as a host notification: a
// delimited block for the user and a machine-parseable summary (with per-task
// retrieval instructions) for the model.
func notificationResult(cp *runner.CallbackPayload) *toolspec.Result {
	failed := cp.Event != "completed"

	// Explicit done/failed/total counts so a partial-failure run is unmistakable
	// at a glance, without scanning every task line. "done" covers terminal
	// success (including "done (QA failed)"); any other non-done terminal status
	// (e.g. "escalate") is neither done nor failed, so the total keeps the picture
	// complete.
	total := len(cp.Tasks)
	done, failedCount := 0, 0
	for _, t := range cp.Tasks {
		switch {
		case strings.HasPrefix(t.Status, "done"):
			done++
		case t.Status == "failed":
			failedCount++
		}
	}
	counts := fmt.Sprintf("%d done, %d failed of %d task(s)", done, failedCount, total)

	var u strings.Builder
	u.WriteString("━━━ TASK NOTIFICATION ━━━\n")
	if failed {
		fmt.Fprintf(&u, "Maestro taskset '%s' (project '%s') finished with failures: %s.\n", cp.Path, cp.Project, counts)
		if cp.ErrorMessage != "" {
			fmt.Fprintf(&u, "Error: %s\n", cp.ErrorMessage)
		}
	} else {
		fmt.Fprintf(&u, "Maestro taskset '%s' (project '%s') completed: %s.\n", cp.Path, cp.Project, counts)
	}
	u.WriteString("━━━")

	var l strings.Builder
	fmt.Fprintf(&l, "[TASK NOTIFICATION] Maestro taskset '%s' in project '%s' finished — event: %s (%s).\n",
		cp.Path, cp.Project, cp.Event, counts)
	if failed && cp.ErrorMessage != "" {
		fmt.Fprintf(&l, "Failure: %s (%s)\n", cp.ErrorMessage, cp.ErrorCode)
	}
	for _, t := range cp.Tasks {
		fmt.Fprintf(&l, "- task %d '%s': %s", t.ID, t.Title, t.Status)
		if t.Error != "" {
			fmt.Fprintf(&l, " (error: %s)", t.Error)
		}
		fmt.Fprintf(&l, ". %s\n", t.RetrievalInstruction)
	}

	return &toolspec.Result{
		ForLLM:  l.String(),
		ForUser: u.String(),
		IsError: failed,
	}
}
