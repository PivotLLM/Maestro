// Maestro
// License: MIT

package maestro

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/logging"
	"github.com/PivotLLM/Maestro/reference"
)

// TestReferenceBuiltinNames checks that BuiltinNames reports exactly the
// top-level entries of the embedded reference library.
func TestReferenceBuiltinNames(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("docs", "ai"))
	if err != nil {
		t.Fatalf("read docs/ai: %v", err)
	}
	var want []string
	for _, e := range entries {
		want = append(want, e.Name())
	}
	slices.Sort(want)

	got := reference.BuiltinNames(EmbeddedReference)
	if !slices.Equal(got, want) {
		t.Errorf("BuiltinNames() = %v, want %v", got, want)
	}
	for _, name := range []string{"templates", "phases", "playbook-examples", "start.md"} {
		if !slices.Contains(got, name) {
			t.Errorf("BuiltinNames() missing %q", name)
		}
	}
}

// TestReferenceMountClashKeepsBuiltin checks that an external directory
// mounted under a built-in name is ignored, so the built-in stays reachable.
func TestReferenceMountClashKeepsBuiltin(t *testing.T) {
	const builtinFile = "templates/default-qa-report.md"
	want, err := EmbeddedReference.ReadFile("docs/ai/" + builtinFile)
	if err != nil {
		t.Fatalf("read embedded %s: %v", builtinFile, err)
	}

	clashDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(clashDir, "default-qa-report.md"), []byte("shadow from Bob"), 0o600); err != nil {
		t.Fatalf("write shadow file: %v", err)
	}
	userDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(userDir, "notes.md"), []byte("notes from Alice"), 0o600); err != nil {
		t.Fatalf("write user file: %v", err)
	}

	svc := reference.NewService(
		reference.WithEmbeddedFS(EmbeddedReference),
		reference.WithExternalDirs([]reference.ExternalDir{
			{Path: clashDir, Mount: "templates"},
			{Path: userDir, Mount: "user"},
		}),
		reference.WithLogger(logging.NewWithWriter(io.Discard)),
	)

	item, err := svc.Get(builtinFile, 0, 0)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", builtinFile, err)
	}
	if item.Content != string(want) {
		t.Errorf("Get(%s) returned the external file, want the built-in", builtinFile)
	}

	item, err = svc.Get("user/notes.md", 0, 0)
	if err != nil {
		t.Fatalf("Get(user/notes.md) error = %v", err)
	}
	if item.Content != "notes from Alice" {
		t.Errorf("Get(user/notes.md) = %q, want the non-clashing mount's file", item.Content)
	}

	items, _, err := svc.Search("shadow from Bob", 10, 0)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	for _, it := range items {
		if strings.HasPrefix(it.Path, "templates/") {
			t.Errorf("Search() found %s from the clashing directory", it.Path)
		}
	}
}
