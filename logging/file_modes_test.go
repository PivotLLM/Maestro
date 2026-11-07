//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package logging

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestNew_FileModes: New creates the log directory and file with the given
// modes, and treats a zero mode as the owner-only default, under a narrowing
// umask and under umask 0.
func TestNew_FileModes(t *testing.T) {
	tests := []struct {
		name              string
		dirMode, fileMode fs.FileMode
		wantDir, wantFile fs.FileMode
	}{
		{"configured", 0o770, 0o660, 0o770, 0o660},
		{"zero means default", 0, 0, 0o700, 0o600},
	}
	for _, mask := range testutil.Umasks {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("umask %04o/%s", mask, tt.name), func(t *testing.T) {
				testutil.SetUmask(t, mask)
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
}
