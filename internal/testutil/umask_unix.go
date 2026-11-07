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

// NarrowUmask sets the process umask to 0o077 for the rest of the test and
// restores the previous value on cleanup, so a mode that loses its group or
// other bits shows the umask was not overridden. The umask is process-wide:
// tests that use this must not run in parallel.
func NarrowUmask(t testing.TB) {
	t.Helper()
	old := syscall.Umask(0o077)
	t.Cleanup(func() { syscall.Umask(old) })
}

// Umasks are the umasks the file-mode tests run under: a narrowing one, so a
// mode that keeps group or other bits shows the umask was overridden, and
// zero, so an owner-only mode shows it was set exactly rather than derived
// from the umask.
var Umasks = []int{0o077, 0}

// SetUmask sets the process umask to mask for the rest of the test and
// restores the previous value on cleanup. The umask is process-wide: tests
// that use this must not run in parallel.
func SetUmask(t testing.TB, mask int) {
	t.Helper()
	old := syscall.Umask(mask)
	t.Cleanup(func() { syscall.Umask(old) })
}
