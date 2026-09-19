/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PivotLLM/Maestro/global"
)

func boolPtr(b bool) *bool { return &b }

// TestWithRunner_AppliedAndDefaulted: a host-supplied runner config reaches
// Runner() with Maestro's defaults filling whatever the host left at zero.
func TestWithRunner_AppliedAndDefaulted(t *testing.T) {
	c := New(WithBaseDir(t.TempDir()), WithRunner(Runner{
		MaxConcurrent: 2,
		RateLimit:     RateLimit{MaxRequests: 3},
		AllowParallel: boolPtr(false),
	}))
	if err := c.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	r := c.Runner()
	if r.MaxConcurrent != 2 {
		t.Errorf("MaxConcurrent = %d, want 2", r.MaxConcurrent)
	}
	if r.RateLimit.MaxRequests != 3 || r.RateLimit.PeriodSeconds != global.DefaultRateLimitPeriod {
		t.Errorf("RateLimit = %+v, want MaxRequests 3 and default period", r.RateLimit)
	}
	if r.MaxRounds != global.DefaultMaxRounds || r.Limits.MaxWorker != global.DefaultMaxWorker {
		t.Errorf("defaults not applied: rounds=%d worker=%d", r.MaxRounds, r.Limits.MaxWorker)
	}
	if r.ParallelAllowed() {
		t.Error("ParallelAllowed() = true, want false")
	}
}

// TestRunner_ParallelAllowed_DefaultTrue: unset (and Prepare without WithRunner)
// permits parallel execution, matching Maestro's existing behaviour.
func TestRunner_ParallelAllowed_DefaultTrue(t *testing.T) {
	c := New(WithBaseDir(t.TempDir()))
	if err := c.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if !c.Runner().ParallelAllowed() {
		t.Error("unset AllowParallel must permit parallel execution")
	}
	if !(Runner{AllowParallel: boolPtr(true)}).ParallelAllowed() {
		t.Error("explicit true must permit parallel execution")
	}
}

// TestWithReferenceDirs_ResolvedByPrepare: host-supplied reference dirs are
// validated, created if missing and exposed through ReferenceDirs().
func TestWithReferenceDirs_ResolvedByPrepare(t *testing.T) {
	ext := filepath.Join(t.TempDir(), "standards")
	c := New(WithBaseDir(t.TempDir()), WithReferenceDirs([]ReferenceDir{{Path: ext, Mount: "standards"}}))
	if err := c.Prepare(); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	dirs := c.ReferenceDirs()
	if len(dirs) != 1 || dirs[0].Mount != "standards" || dirs[0].Path != ext {
		t.Fatalf("ReferenceDirs() = %+v, want one entry for %s", dirs, ext)
	}
	if st, err := os.Stat(ext); err != nil || !st.IsDir() {
		t.Errorf("reference dir was not created: %v", err)
	}
}

// TestWithReferenceDirs_ValidationStillApplies: the same rules as the config
// file entry apply to programmatic entries.
func TestWithReferenceDirs_ValidationStillApplies(t *testing.T) {
	for _, bad := range []ReferenceDir{
		{Path: t.TempDir(), Mount: "a/b"},
		{Path: t.TempDir(), Mount: ".."},
		{Path: "", Mount: "x"},
		{Path: t.TempDir(), Mount: ""},
	} {
		c := New(WithBaseDir(t.TempDir()), WithReferenceDirs([]ReferenceDir{bad}))
		if err := c.Prepare(); err == nil || !strings.Contains(err.Error(), "reference_dirs") {
			t.Errorf("Prepare with %+v: err = %v, want a reference_dirs validation error", bad, err)
		}
	}
}
