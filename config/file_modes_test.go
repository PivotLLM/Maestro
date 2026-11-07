//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package config

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/PivotLLM/Maestro/global"
	"github.com/PivotLLM/Maestro/internal/testutil"
)

// TestWithFileModes: the option overrides each mode independently and a zero
// value (or no option) keeps the owner-only default.
func TestWithFileModes(t *testing.T) {
	if global.DefaultDirMode != 0o700 || global.DefaultFileMode != 0o600 {
		t.Fatalf("defaults = %o/%o, want 700/600", global.DefaultDirMode, global.DefaultFileMode)
	}
	tests := []struct {
		name     string
		opts     []Option
		wantDir  fs.FileMode
		wantFile fs.FileMode
	}{
		{"no option", nil, global.DefaultDirMode, global.DefaultFileMode},
		{"both set", []Option{WithFileModes(0o770, 0o660)}, 0o770, 0o660},
		{"zero dir keeps default", []Option{WithFileModes(0, 0o660)}, global.DefaultDirMode, 0o660},
		{"zero file keeps default", []Option{WithFileModes(0o770, 0)}, 0o770, global.DefaultFileMode},
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
// the configured mode, or exactly the owner-only default without the option,
// under a narrowing umask and under umask 0.
func TestWithFileModes_PrepareCreatesDirs(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want fs.FileMode
	}{
		{"configured", []Option{WithFileModes(0o770, 0o660)}, 0o770},
		{"default", nil, 0o700},
	}
	for _, mask := range testutil.Umasks {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("umask %04o/%s", mask, tt.name), func(t *testing.T) {
				testutil.SetUmask(t, mask)
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
}
