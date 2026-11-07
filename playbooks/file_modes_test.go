//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package playbooks

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestWithFileModes_ReachCreatedFiles: the modes from WithFileModes (or exactly
// the owner-only defaults without it) reach the playbook directory, its
// subdirectories, files and metadata, under a narrowing umask and under umask 0.
func TestWithFileModes_ReachCreatedFiles(t *testing.T) {
	tests := []struct {
		name              string
		opts              []Option
		wantDir, wantFile fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o770, 0o660)}, 0o770, 0o660},
		{"default", nil, 0o700, 0o600},
	}
	for _, mask := range testutil.Umasks {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("umask %04o/%s", mask, tt.name), func(t *testing.T) {
				testutil.SetUmask(t, mask)
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
}
