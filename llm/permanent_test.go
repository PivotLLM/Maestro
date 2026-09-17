/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestPermanent_WrapsAndIsDetectable(t *testing.T) {
	base := errors.New("maximum sub-agent depth reached")
	perm := Permanent(base)

	if !IsPermanent(perm) {
		t.Fatal("IsPermanent(Permanent(err)) = false, want true")
	}
	if perm.Error() != base.Error() {
		t.Errorf("Error() = %q, want %q", perm.Error(), base.Error())
	}
	if !errors.Is(perm, base) {
		t.Error("errors.Is(perm, base) = false; PermanentError must unwrap to the cause")
	}
	// Still detectable when wrapped further up the stack.
	wrapped := fmt.Errorf("dispatch failed: %w", perm)
	if !IsPermanent(wrapped) {
		t.Error("IsPermanent(fmt.Errorf(%%w)) = false, want true")
	}
}

func TestPermanent_NilAndOrdinaryErrors(t *testing.T) {
	if Permanent(nil) != nil {
		t.Error("Permanent(nil) must be nil")
	}
	if IsPermanent(nil) {
		t.Error("IsPermanent(nil) = true, want false")
	}
	if IsPermanent(errors.New("transient")) {
		t.Error("an ordinary error must not be permanent")
	}
}
