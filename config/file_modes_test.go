//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package config

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestWithFileModes: the option overrides each mode independently and a zero
// value (or no option) keeps the default.
func TestWithFileModes(t *testing.T) {
	tests := []struct {
		name     string
		opts     []Option
		wantDir  fs.FileMode
		wantFile fs.FileMode
	}{
		{"no option", nil, global.DefaultDirMode, global.DefaultFileMode},
		{"both set", []Option{WithFileModes(0o750, 0o640)}, 0o750, 0o640},
		{"zero dir keeps default", []Option{WithFileModes(0, 0o640)}, global.DefaultDirMode, 0o640},
		{"zero file keeps default", []Option{WithFileModes(0o750, 0)}, 0o750, global.DefaultFileMode},
		{"both zero", []Option{WithFileModes(0, 0)}, global.DefaultDirMode, global.DefaultFileMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.opts...)
			if got := c.DirMode(); got != tt.wantDir {
				t.Errorf("DirMode() = %o, want %o", got, tt.wantDir)
			}
			if got := c.FileMode(); got != tt.wantFile {
				t.Errorf("FileMode() = %o, want %o", got, tt.wantFile)
			}
		})
	}
}

// TestWithFileModes_PrepareCreatesDirs: the directories Prepare creates get
// the configured mode, or exactly the default without the option.
func TestWithFileModes_PrepareCreatesDirs(t *testing.T) {
	testutil.ZeroUmask(t)
	tests := []struct {
		name string
		opts []Option
		want fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o750, 0o640)}, 0o750},
		{"default", nil, 0o755},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(append([]Option{WithBaseDir(filepath.Join(t.TempDir(), "base"))}, tt.opts...)...)
			if err := c.Prepare(); err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			for _, dir := range []string{c.BaseDir(), c.PlaybooksDir(), c.ProjectsDir(), c.AgentsDir()} {
				testutil.AssertPerm(t, dir, tt.want)
			}
		})
	}
}
