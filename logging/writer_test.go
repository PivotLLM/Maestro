/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package logging

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/global"
)

// lineFormat is the contract a host relies on to extract the level from each
// line: timestamp, bracketed level, bracketed pid, message.
var lineFormat = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} \[(DEBUG|INFO|WARN|ERROR|FATAL)\] \[\d+\] (.*)$`)

func TestNewWithWriter_LineFormatAndLevels(t *testing.T) {
	var buf bytes.Buffer
	l := NewWithWriter(&buf)

	l.Debugf("hidden %d", 1) // below the default INFO level
	l.Infof("task %d started", 7)
	l.Warn("careful")
	l.Errorf("failed: %s", "boom")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (debug suppressed):\n%s", len(lines), buf.String())
	}
	want := []struct{ level, msg string }{{"INFO", "task 7 started"}, {"WARN", "careful"}, {"ERROR", "failed: boom"}}
	for i, line := range lines {
		m := lineFormat.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("line %d %q does not match the documented format", i, line)
		}
		if m[1] != want[i].level || m[2] != want[i].msg {
			t.Errorf("line %d = %s %q, want %s %q", i, m[1], m[2], want[i].level, want[i].msg)
		}
	}

	buf.Reset()
	l.SetLevel(global.LogLevelDebug)
	l.Debug("now visible")
	if !strings.Contains(buf.String(), "[DEBUG] ") || !strings.HasSuffix(strings.TrimSpace(buf.String()), "now visible") {
		t.Errorf("debug line after SetLevel = %q", buf.String())
	}

	if err := l.Sync(); err != nil {
		t.Errorf("Sync on writer logger: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Errorf("Close on writer logger: %v", err)
	}
}
