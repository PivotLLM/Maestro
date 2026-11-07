//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package library

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
	"github.com/PivotLLM/Maestro/logging"
)

// TestWithFileModes_ReachCreatedFiles: put, append and rename create folders,
// files and metadata with exactly the modes from WithFileModes, or exactly the
// owner-only defaults without it (a zero value keeps the default), under a
// narrowing umask and under umask 0.
func TestWithFileModes_ReachCreatedFiles(t *testing.T) {
	tests := []struct {
		name              string
		opts              []Option
		wantDir, wantFile fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o770, 0o660)}, 0o770, 0o660},
		{"zero keeps default", []Option{WithFileModes(0, 0)}, 0o700, 0o600},
		{"default", nil, 0o700, 0o600},
	}
	for _, mask := range testutil.Umasks {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("umask %04o/%s", mask, tt.name), func(t *testing.T) {
				testutil.SetUmask(t, mask)
				base := t.TempDir()
				lg, err := logging.New(filepath.Join(base, "logs", "test.log"), 0, 0)
				if err != nil {
					t.Fatalf("logger: %v", err)
				}
				t.Cleanup(func() { _ = lg.Close() })
				catDir := filepath.Join(base, "docs")
				svc := NewService(lg, []config.Category{{Name: "docs", Directory: catDir}}, tt.opts...)

				if _, _, err := svc.PutItem("docs/alice/notes.md", "hello", "summary", false); err != nil {
					t.Fatalf("PutItem: %v", err)
				}
				if _, err := svc.AppendItem("docs/bob/log.md", "entry"); err != nil {
					t.Fatalf("AppendItem: %v", err)
				}
				if _, _, err := svc.RenameItem("docs/alice/notes.md", "docs/moved/deep/notes.md", false); err != nil {
					t.Fatalf("RenameItem: %v", err)
				}

				for _, dir := range []string{
					catDir,
					filepath.Join(catDir, "alice"),
					filepath.Join(catDir, "bob"),
					filepath.Join(catDir, "moved"),
					filepath.Join(catDir, "moved", "deep"),
				} {
					testutil.AssertPerm(t, dir, tt.wantDir)
				}
				for _, file := range []string{
					filepath.Join(catDir, "bob", "log.md"),
					filepath.Join(catDir, "bob", "log.md"+global.MetaSuffix),
					filepath.Join(catDir, "moved", "deep", "notes.md"),
				} {
					testutil.AssertPerm(t, file, tt.wantFile)
				}
			})
		}
	}
}
