//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package library

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
	"github.com/PivotLLM/Maestro/logging"
)

// TestDefaultModes_ReachCreatedFiles: the library always uses the owner-only
// defaults: put, append and rename create folders 0700 and files (with their
// metadata) 0600, under a narrowing umask and under umask 0.
func TestDefaultModes_ReachCreatedFiles(t *testing.T) {
	for _, mask := range testutil.Umasks {
		t.Run(fmt.Sprintf("umask %04o", mask), func(t *testing.T) {
			testutil.SetUmask(t, mask)
			base := t.TempDir()
			lg, err := logging.New(filepath.Join(base, "logs", "test.log"), 0, 0)
			if err != nil {
				t.Fatalf("logger: %v", err)
			}
			t.Cleanup(func() { _ = lg.Close() })
			catDir := filepath.Join(base, "docs")
			svc := &Service{
				logger:     lg,
				categories: map[string]*config.Category{"docs": {Name: "docs", Directory: catDir}},
			}

			if _, _, err := svc.PutItem("docs/alice/notes.md", "hello", "summary", false); err != nil {
				t.Fatalf("PutItem: %v", err)
			}
			if _, err := svc.AppendItem("docs/bob/log.md", "entry"); err != nil {
				t.Fatalf("AppendItem: %v", err)
			}
			if _, _, err := svc.RenameItem("docs/alice/notes.md", "docs/moved/notes.md", false); err != nil {
				t.Fatalf("RenameItem: %v", err)
			}

			for _, dir := range []string{
				catDir,
				filepath.Join(catDir, "alice"),
				filepath.Join(catDir, "bob"),
				filepath.Join(catDir, "moved"),
			} {
				testutil.AssertPerm(t, dir, 0o700)
			}
			for _, file := range []string{
				filepath.Join(catDir, "bob", "log.md"),
				filepath.Join(catDir, "moved", "notes.md"),
			} {
				testutil.AssertPerm(t, file, 0o600)
			}
			testutil.AssertPerm(t, filepath.Join(catDir, "bob", "log.md"+global.MetaSuffix), 0o600)
		})
	}
}
