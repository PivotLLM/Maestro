package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPrepare_ProgrammaticBaseDir verifies the embedding path: WithBaseDir +
// Prepare() configures Maestro without a config file, deriving and creating the
// standard subdirectories under the host-supplied base.
func TestPrepare_ProgrammaticBaseDir(t *testing.T) {
	base := filepath.Join(t.TempDir(), "maestro")
	c := New(WithBaseDir(base))
	if err := c.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if c.BaseDir() != base {
		t.Errorf("BaseDir = %q, want %q", c.BaseDir(), base)
	}
	if got, want := c.ProjectsDir(), filepath.Join(base, "projects"); got != want {
		t.Errorf("ProjectsDir = %q, want %q", got, want)
	}
	if got, want := c.PlaybooksDir(), filepath.Join(base, "playbooks"); got != want {
		t.Errorf("PlaybooksDir = %q, want %q", got, want)
	}
	// The subdirectories are created on disk.
	for _, d := range []string{c.ProjectsDir(), c.PlaybooksDir()} {
		if fi, err := os.Stat(d); err != nil || !fi.IsDir() {
			t.Errorf("expected %q to be a created directory (err=%v)", d, err)
		}
	}
}
