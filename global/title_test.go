/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import "testing"

func TestTitle_MatchesFormerStringsTitle(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"hello world":         "Hello World",
		"already Title":       "Already Title",
		"under_score words":   "Under_score Words",
		"hyphen-ated and 3rd": "Hyphen-Ated And 3rd",
		"ünicode wörds":       "Ünicode Wörds",
	}
	for in, want := range cases {
		if got := Title(in); got != want {
			t.Errorf("Title(%q) = %q, want %q", in, got, want)
		}
	}
}
