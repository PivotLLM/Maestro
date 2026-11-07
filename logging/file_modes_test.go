//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package logging

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestNew_FileModes: New creates the log directory and file with the given
// modes, and treats a zero mode as the default.
func TestNew_FileModes(t *testing.T) {
	testutil.ZeroUmask(t)
	tests := []struct {
		name              string
		dirMode, fileMode fs.FileMode
		wantDir, wantFile fs.FileMode
	}{
		{"configured", 0o750, 0o640, 0o750, 0o640},
		{"zero means default", 0, 0, 0o755, 0o644},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "logs", "test.log")
			lg, err := New(logPath, tt.dirMode, tt.fileMode)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			t.Cleanup(func() { _ = lg.Close() })
			testutil.AssertPerm(t, filepath.Dir(logPath), tt.wantDir)
			testutil.AssertPerm(t, logPath, tt.wantFile)
		})
	}
}
