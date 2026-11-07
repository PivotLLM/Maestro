//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package lists

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestWithFileModes_ReachCreatedFiles: the modes from WithFileModes (or exactly
// the owner-only defaults without it) reach the lists directory and the list
// file, under a narrowing umask and under umask 0.
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
				service, tempDir := setupTestService(t)
				t.Cleanup(func() { _ = os.RemoveAll(tempDir) })
				for _, opt := range tt.opts {
					opt(service)
				}
				createTestProject(t, tempDir, "alice")

				items := []global.ListItem{{ID: "item-1", Title: "First", Content: "First item"}}
				if err := service.Create(SourceProject, "alice", "", "bob.json", "Bob", "", items); err != nil {
					t.Fatalf("Create: %v", err)
				}

				listsDir := filepath.Join(tempDir, "projects", "alice", global.ListsDir)
				testutil.AssertPerm(t, listsDir, tt.wantDir)
				testutil.AssertPerm(t, filepath.Join(listsDir, "bob.json"), tt.wantFile)
			})
		}
	}
}
