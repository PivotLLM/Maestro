//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package reporting

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestSaveReport_FileModes: SaveReport creates the report folder and file with
// exactly the modes from WithFileModes, or exactly the owner-only defaults
// without it (a zero value keeps the default), under a narrowing umask and
// under umask 0.
func TestSaveReport_FileModes(t *testing.T) {
	tests := []struct {
		name              string
		opts              []Option
		wantDir, wantFile fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o770, 0o660)}, 0o770, 0o660},
		{"zero keeps default", []Option{WithFileModes(0, 0)}, 0o700, 0o600},
		{"default", nil, 0o700, 0o600},
	}
	report := &ProjectReport{Project: "alice", GeneratedAt: time.Now()}
	for _, mask := range testutil.Umasks {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("umask %04o/%s", mask, tt.name), func(t *testing.T) {
				testutil.SetUmask(t, mask)
				r := New(nil, tt.opts...)
				outDir := filepath.Join(t.TempDir(), "reports", "bob")
				outPath := filepath.Join(outDir, "report.md")
				if err := r.SaveReport(report, outPath, "markdown"); err != nil {
					t.Fatalf("SaveReport: %v", err)
				}
				testutil.AssertPerm(t, filepath.Dir(outDir), tt.wantDir)
				testutil.AssertPerm(t, outDir, tt.wantDir)
				testutil.AssertPerm(t, outPath, tt.wantFile)
			})
		}
	}
}
