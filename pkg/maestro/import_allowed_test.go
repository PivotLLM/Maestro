// Maestro
// License: MIT

package maestro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/toolspec"

	"github.com/PivotLLM/Maestro/global"
)

func importSourceDescription(defs []toolspec.ToolDefinition) string {
	for _, d := range defs {
		if d.Name == global.ToolFileImport {
			for _, p := range d.Parameters {
				if p.Name == "source" {
					return p.Description
				}
			}
		}
	}
	return ""
}

// TestHostDeps_ImportAllowed_Wired: a host predicate reaches file_import — a
// refused path errors, a permitted one imports — and the tool's source
// description no longer promises "anywhere on the filesystem".
func TestHostDeps_ImportAllowed_Wired(t *testing.T) {
	allowed := t.TempDir()
	p := &Provider{}
	cfg := newPreparedConfig(t)
	defs := p.RegisterTools(toolspec.Deps{Cfg: cfg, Host: HostDeps{
		Dispatcher:    stubDispatcher{},
		ImportAllowed: func(abs string) bool { return strings.HasPrefix(abs, allowed+string(os.PathSeparator)) },
	}})

	if desc := importSourceDescription(defs); strings.Contains(desc, "anywhere") || !strings.Contains(desc, "may already read") {
		t.Errorf("host file_import source description = %q", desc)
	}

	if _, err := p.projects.Create("imp", "Import", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := p.handleFileImport(&toolspec.ToolCall{Args: map[string]any{"project": "imp", "source": outside}})
	if err != nil {
		t.Fatalf("handleFileImport: %v", err)
	}
	if !res.IsError || !strings.Contains(res.ForLLM, "import refused") {
		t.Errorf("outside import result = %+v, want refusal", res)
	}

	inside := filepath.Join(allowed, "ok.txt")
	if err := os.WriteFile(inside, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err = p.handleFileImport(&toolspec.ToolCall{Args: map[string]any{"project": "imp", "source": inside}})
	if err != nil || res.IsError {
		t.Fatalf("inside import: res=%+v err=%v", res, err)
	}
}

// TestHostDeps_NoImportAllowed_StandaloneWording: without a predicate the
// standalone description and behaviour are unchanged.
func TestHostDeps_NoImportAllowed_StandaloneWording(t *testing.T) {
	p := &Provider{}
	defs := p.RegisterTools(toolspec.Deps{Cfg: newPreparedConfig(t), Host: HostDeps{Dispatcher: stubDispatcher{}}})
	if desc := importSourceDescription(defs); !strings.Contains(desc, "absolute path on the filesystem") {
		t.Errorf("standalone file_import source description changed: %q", desc)
	}
}

// TestHostDeps_ImportAllowed_RefusedCountSurfaced: entries the host refuses
// inside a directory import are reported to the caller as files_refused.
func TestHostDeps_ImportAllowed_RefusedCountSurfaced(t *testing.T) {
	allowed := t.TempDir()
	p := &Provider{}
	p.RegisterTools(toolspec.Deps{Cfg: newPreparedConfig(t), Host: HostDeps{
		Dispatcher:    stubDispatcher{},
		ImportAllowed: func(abs string) bool { return strings.HasPrefix(abs, allowed+string(os.PathSeparator)) },
	}})
	if _, err := p.projects.Create("imp", "Import", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	dir := filepath.Join(allowed, "docs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.md"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "leak")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	res, err := p.handleFileImport(&toolspec.ToolCall{Args: map[string]any{"project": "imp", "source": dir, "recursive": true}})
	if err != nil || res.IsError {
		t.Fatalf("import: res=%+v err=%v", res, err)
	}
	for _, want := range []string{`"files_imported":1`, `"files_refused":1`} {
		if !strings.Contains(res.ForLLM, want) {
			t.Errorf("result lacks %s: %s", want, res.ForLLM)
		}
	}
}
