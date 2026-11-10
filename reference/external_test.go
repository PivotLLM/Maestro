/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package reference

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const outsideSecret = "outside-secret-marker"

// newSymlinkService builds a service with one external directory mounted as
// "user". The directory holds a plain file, a link to it, and links that lead
// outside the directory to a file and to a folder holding a secret.
func newSymlinkService(t *testing.T) *Service {
	t.Helper()
	base := t.TempDir()

	outside := filepath.Join(base, "outside")
	refDir := filepath.Join(base, "ref")
	for _, dir := range []string{outside, refDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte(outsideSecret), 0o600); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	if err := os.WriteFile(filepath.Join(refDir, "inside.txt"), []byte("notes for Alice"), 0o600); err != nil {
		t.Fatalf("write inside: %v", err)
	}

	links := map[string]string{
		"link-in.txt":  "inside.txt",
		"link-out.txt": filepath.Join(outside, "secret.txt"),
		"rel-out.txt":  "../outside/secret.txt",
		"dir-out":      outside,
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(refDir, name)); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}

	return &Service{
		fs:           testFS,
		prefix:       "testdata",
		externalDirs: []ExternalDir{{Path: refDir, Mount: "user"}},
		logger:       createTestLogger(t),
	}
}

func TestExternalGetSymlinks(t *testing.T) {
	svc := newSymlinkService(t)

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "file inside the directory", path: "user/inside.txt", want: "notes for Alice"},
		{name: "link inside the directory", path: "user/link-in.txt", want: "notes for Alice"},
		{name: "absolute link outside", path: "user/link-out.txt", wantErr: true},
		{name: "relative link outside", path: "user/rel-out.txt", wantErr: true},
		{name: "file under a linked outside folder", path: "user/dir-out/secret.txt", wantErr: true},
		{name: "dot-dot traversal", path: "user/../outside/secret.txt", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := svc.Get(tt.path, 0, 0)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Get(%q) returned %q, want an error", tt.path, item.Content)
				}
				if strings.Contains(err.Error(), outsideSecret) {
					t.Errorf("Get(%q) error leaks outside content: %v", tt.path, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Get(%q) error = %v", tt.path, err)
			}
			if item.Content != tt.want {
				t.Errorf("Get(%q) content = %q, want %q", tt.path, item.Content, tt.want)
			}
		})
	}
}

func TestExternalListSkipsEscapingLinks(t *testing.T) {
	svc := newSymlinkService(t)

	items, err := svc.List("user/")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	got := map[string]bool{}
	for _, item := range items {
		got[item.Path] = true
	}
	for _, want := range []string{"user/inside.txt", "user/link-in.txt"} {
		if !got[want] {
			t.Errorf("List() missing %s; got %v", want, got)
		}
	}
	for _, unwanted := range []string{"user/link-out.txt", "user/rel-out.txt", "user/dir-out", "user/dir-out/secret.txt"} {
		if got[unwanted] {
			t.Errorf("List() includes %s, which leads outside the directory", unwanted)
		}
	}
}

func TestExternalSearchSkipsEscapingLinks(t *testing.T) {
	svc := newSymlinkService(t)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "content behind escaping links", query: outsideSecret, want: nil},
		{name: "names of escaping links", query: "-out", want: nil},
		{name: "content inside the directory", query: "alice", want: []string{"user/inside.txt", "user/link-in.txt"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total, err := svc.Search(tt.query, 10, 0)
			if err != nil {
				t.Fatalf("Search(%q) error = %v", tt.query, err)
			}
			var got []string
			for _, item := range items {
				got = append(got, item.Path)
			}
			if total != len(tt.want) || strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("Search(%q) = %v (total %d), want %v", tt.query, got, total, tt.want)
			}
		})
	}
}
