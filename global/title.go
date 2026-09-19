/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package global

import (
	"strings"
	"unicode"
)

// Title returns s with the first letter of every word upper-cased, using the
// word-boundary rule strings.Title used (a letter or digit following a
// separator), which the standard library deprecated without a drop-in
// replacement. Used for report headings and template functions.
func Title(s string) string {
	prev := ' '
	return strings.Map(func(r rune) rune {
		if isSeparator(prev) {
			prev = r
			return unicode.ToTitle(r)
		}
		prev = r
		return r
	}, s)
}

// isSeparator mirrors the rule behind strings.Title: ASCII letters, digits and
// underscore continue a word; other ASCII is a separator; beyond ASCII, letters
// and digits continue a word and only spaces separate.
func isSeparator(r rune) bool {
	if r <= 0x7F {
		switch {
		case '0' <= r && r <= '9', 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', r == '_':
			return false
		}
		return true
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return false
	}
	return unicode.IsSpace(r)
}
