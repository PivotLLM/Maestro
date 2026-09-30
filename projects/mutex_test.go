/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package projects

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/PivotLLM/Maestro/config"
	"github.com/PivotLLM/Maestro/logging"
)

// newServiceOnBaseDir builds a Service over baseDir, as an embedding host does
// on each provider registration.
func newServiceOnBaseDir(t *testing.T, baseDir string) *Service {
	t.Helper()
	cfg := config.New(config.WithBaseDir(baseDir))
	if err := cfg.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	lg, err := logging.New(filepath.Join(t.TempDir(), "test.log"))
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	t.Cleanup(func() { _ = lg.Close() })
	return NewService(cfg, lg)
}

// addToManifestAsync runs AddToManifest on svc in a goroutine and returns a
// channel that receives its error when it finishes.
func addToManifestAsync(svc *Service, project, taskset string) <-chan error {
	done := make(chan error, 1)
	go func() {
		_, err := svc.AddToManifest(project, taskset)
		done <- err
	}()
	return done
}

// TestProjectMutex_SharedAcrossServicesOnSameBaseDir: two Services over the
// same base directory exclude each other on project.json writes.
func TestProjectMutex_SharedAcrossServicesOnSameBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	a := newServiceOnBaseDir(t, baseDir)
	b := newServiceOnBaseDir(t, baseDir)
	if _, err := a.Create("shared", "Shared", "", "", "", "none"); err != nil {
		t.Fatalf("create project: %v", err)
	}

	if a.GetMutex("shared") != b.GetMutex("shared") {
		t.Fatal("services on the same base dir returned different mutexes")
	}

	mu := a.GetMutex("shared")
	mu.Lock()
	done := addToManifestAsync(b, "shared", "ts1")

	select {
	case err := <-done:
		mu.Unlock()
		t.Fatalf("AddToManifest on the second service did not wait for the lock (err=%v)", err)
	case <-time.After(200 * time.Millisecond):
	}

	mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("AddToManifest: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AddToManifest did not finish after the lock was released")
	}
}

// TestProjectMutex_IndependentAcrossBaseDirs: a project of the same name under
// a different base directory has its own lock.
func TestProjectMutex_IndependentAcrossBaseDirs(t *testing.T) {
	a := newServiceOnBaseDir(t, t.TempDir())
	b := newServiceOnBaseDir(t, t.TempDir())
	for _, svc := range []*Service{a, b} {
		if _, err := svc.Create("same-name", "Same", "", "", "", "none"); err != nil {
			t.Fatalf("create project: %v", err)
		}
	}

	if a.GetMutex("same-name") == b.GetMutex("same-name") {
		t.Fatal("services on different base dirs share a mutex")
	}

	mu := a.GetMutex("same-name")
	mu.Lock()
	defer mu.Unlock()

	select {
	case err := <-addToManifestAsync(b, "same-name", "ts1"):
		if err != nil {
			t.Fatalf("AddToManifest: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AddToManifest on another base dir blocked on an unrelated lock")
	}
}
