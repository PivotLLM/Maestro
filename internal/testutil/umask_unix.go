//go:build unix

/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

// Package testutil holds helpers shared by Maestro's tests.
package testutil

import (
	"syscall"
	"testing"
)

// ZeroUmask sets the process umask to 0 for the rest of the test and restores
// the previous value on cleanup, so created files get exactly the mode passed
// to the call that created them. The umask is process-wide: tests that use
// this must not run in parallel.
func ZeroUmask(t testing.TB) {
	t.Helper()
	old := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(old) })
}
