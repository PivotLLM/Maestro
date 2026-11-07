//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package playbooks

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestWithFileModes_ReachCreatedFiles: the modes from WithFileModes (or exactly
// the defaults without it) reach the playbook directory, its subdirectories,
// files and metadata.
func TestWithFileModes_ReachCreatedFiles(t *testing.T) {
	testutil.NarrowUmask(t)
	tests := []struct {
		name              string
		opts              []Option
		wantDir, wantFile fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o770, 0o660)}, 0o770, 0o660},
		{"default", nil, 0o755, 0o644},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseDir := t.TempDir()
			svc := NewService(baseDir, createTestLogger(t), tt.opts...)
			if err := svc.Create("alice"); err != nil {
				t.Fatalf("Create: %v", err)
			}
			if _, err := svc.PutFile("alice", "docs/bob.md", "hello", "summary"); err != nil {
				t.Fatalf("PutFile: %v", err)
			}
			testutil.AssertPerm(t, filepath.Join(baseDir, "alice"), tt.wantDir)
			testutil.AssertPerm(t, filepath.Join(baseDir, "alice", "docs"), tt.wantDir)
			testutil.AssertPerm(t, filepath.Join(baseDir, "alice", "docs", "bob.md"), tt.wantFile)
			testutil.AssertPerm(t, filepath.Join(baseDir, "alice", "docs", "bob.md"+global.MetaSuffix), tt.wantFile)
		})
	}
}
