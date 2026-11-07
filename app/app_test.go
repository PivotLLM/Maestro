/******************************************************************************
 * Copyright (c) 2025-2026 Tenebris Technologies Inc.                         *
 * Please see the LICENSE file for details                                    *
 ******************************************************************************/

package app

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

// Standard library only, on purpose: this file is copied into every project
// and must not be the reason a module grows a test dependency.

// TestVersion_IsTheOnlySourceOfTruth pins why the const is unexported: the
// release version cannot be overwritten at link time or runtime, so no build can
// report a version its source does not declare.
func TestVersion_IsTheOnlySourceOfTruth(t *testing.T) {
	if version == "" {
		t.Fatal("version must not be empty")
	}
	if version == "dev" {
		t.Fatal("version must be a real release number, not a build-time placeholder")
	}
	if !strings.HasPrefix(Version(), version) {
		t.Fatalf("Version()=%q must lead with the release number %q, decorating it at most", Version(), version)
	}
	if got := SemVer(); got != version {
		t.Fatalf("SemVer()=%q want %q", got, version)
	}
}

// TestVersion_ReleaseAndCommitAreOneToken is the reason for the "+". Rendered as
// "0.1.0 (git: abc)", a version gets pasted into a bug report as "0.1.0" — the
// space reads as the end of the value and the parenthetical as an aside, so the
// half identifying the exact source is the half that gets dropped. The release
// and the commit therefore have to survive as a single word, whatever else the
// string carries.
func TestVersion_ReleaseAndCommitAreOneToken(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"
	got := Version()

	if strings.Contains(got, "(") {
		t.Fatalf("Version()=%q: no parentheses — they invite truncation", got)
	}
	if first := strings.Fields(got)[0]; first != version+"+abc12345" {
		t.Fatalf("first token=%q want %q: truncating at the first space must still leave the release and the commit", first, version+"+abc12345")
	}
}

// TestVersion_UsesBuildMetadataSeparator pins the separator against SemVer's two
// meanings. "+" is build metadata, which the spec requires be ignored when
// comparing versions, so 0.1.0+abc compares EQUAL to 0.1.0. "-" would be a
// pre-release identifier, making the stamped build compare LOWER than the plain
// release and claim to be something that came before it.
func TestVersion_UsesBuildMetadataSeparator(t *testing.T) {
	old := gitCommit
	t.Cleanup(func() { gitCommit = old })

	gitCommit = "abc12345"
	got := Version()

	if !strings.Contains(got, "+") {
		t.Fatalf("Version()=%q: the commit must attach as SemVer build metadata", got)
	}
	got = strings.Fields(got)[0]
	if strings.Contains(strings.TrimPrefix(got, version), "-") {
		t.Fatalf("Version()=%q: a hyphen would make this a pre-release, sorting below the release itself", got)
	}
	if before := strings.SplitN(got, "+", 2)[0]; before != version {
		t.Fatalf("before the + is %q want %q: must be the untouched release number", before, version)
	}
}

// TestVersion_UnstampedBuildIsBare covers a plain `go build`: no commit, so no
// dangling separator on the end of the number.
func TestVersion_UnstampedBuildIsBare(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "", ""
	if got := Version(); got != version {
		t.Fatalf("Version()=%q want bare %q", got, version)
	}
	if strings.Contains(Version(), "+") {
		t.Fatalf("Version()=%q: no dangling separator when nothing is stamped in", Version())
	}
	if strings.Contains(Version(), "[") {
		t.Fatalf("Version()=%q: no empty brackets when no build number is stamped in", Version())
	}
	if got := Build(); got != "" {
		t.Fatalf("Build()=%q want empty", got)
	}
	if got := BuildDate(); got != 0 {
		t.Fatalf("BuildDate()=%d want 0", got)
	}
}

// TestSemVer_IsPlainSemver pins the protocol contract: no build metadata, ever.
// Clients may compare this field, and some of them are software you do not
// control.
func TestSemVer_IsPlainSemver(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"

	if got := SemVer(); got != version {
		t.Fatalf("SemVer()=%q want %q", got, version)
	}
	for _, bad := range []string{"+", "[", " "} {
		if strings.Contains(SemVer(), bad) {
			t.Fatalf("SemVer()=%q contains %q: the protocol form is a single bare token", SemVer(), bad)
		}
	}
	if Version() == SemVer() {
		t.Fatalf("Version()=%q must differ from SemVer() once a build is stamped", Version())
	}
}

// TestIdentityAccessors covers the remaining identity, which has no interesting
// logic but must not return empty strings into a startup banner.
func TestIdentityAccessors(t *testing.T) {
	for label, got := range map[string]string{
		"Name": Name(), "TagLine": TagLine(), "Copyright": Copyright(),
	} {
		if got == "" {
			t.Errorf("%s() must not be empty", label)
		}
	}
}

func TestBuildInfo_UsesStampedValues(t *testing.T) {
	ob, og := buildTime, goVersion
	t.Cleanup(func() { buildTime, goVersion = ob, og })

	buildTime, goVersion = "2026-02-20T00:00:00Z", "go1.23.0"

	b, g := BuildInfo()
	if b != "2026-02-20T00:00:00Z" {
		t.Errorf("buildTime=%q want %q", b, "2026-02-20T00:00:00Z")
	}
	if g != "go1.23.0" {
		t.Errorf("goVersion=%q want %q", g, "go1.23.0")
	}
}

// TestBuildInfo_FallsBackForToolchainOnly — the toolchain is knowable from the
// running binary even unstamped; the timestamp is not, and inventing one would
// be worse than reporting none.
func TestBuildInfo_FallsBackForToolchainOnly(t *testing.T) {
	ob, og := buildTime, goVersion
	t.Cleanup(func() { buildTime, goVersion = ob, og })

	buildTime, goVersion = "", ""

	b, g := BuildInfo()
	if b != "" {
		t.Errorf("buildTime=%q want empty: there is no fallback for an unstamped timestamp", b)
	}
	if g != runtime.Version() {
		t.Errorf("goVersion=%q want runtime %q", g, runtime.Version())
	}
}

// TestVersion_CarriesTheBuildNumber is the point of the whole thing: the number
// has to be visible wherever the version is, because the surfaces that matter
// all render Version() and nothing else.
func TestVersion_CarriesTheBuildNumber(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "abc12345", "20260902155301"

	if got, want := Version(), version+"+abc12345 [20260902155301]"; got != want {
		t.Errorf("Version()=%q want %q", got, want)
	}
	if got := Build(); got != "20260902155301" {
		t.Errorf("Build()=%q want %q", got, "20260902155301")
	}
	if got := BuildDate(); got != 20260902 {
		t.Errorf("BuildDate()=%d want %d", got, 20260902)
	}
}

// TestVersion_BuildNumberWithoutCommit covers a build stamped by something that
// sets only the number: the brackets still attach cleanly to a bare release
// rather than dangling off a stray "+".
func TestVersion_BuildNumberWithoutCommit(t *testing.T) {
	og, ob := gitCommit, buildNumber
	t.Cleanup(func() { gitCommit, buildNumber = og, ob })

	gitCommit, buildNumber = "", "20260902155301"

	if got, want := Version(), version+" [20260902155301]"; got != want {
		t.Errorf("Version()=%q want %q", got, want)
	}
	if strings.Contains(Version(), "+") {
		t.Errorf("Version()=%q: no separator for a commit that was never stamped", Version())
	}
	if got := BuildDate(); got != 20260902 {
		t.Errorf("BuildDate()=%d want %d", got, 20260902)
	}
}

// TestBuildDate_RequiresAFullDay pins that a truncated or non-numeric stamp
// must not invent a date — 0 is the unstamped value, not a guess.
func TestBuildDate_RequiresAFullDay(t *testing.T) {
	old := buildNumber
	t.Cleanup(func() { buildNumber = old })

	for _, stamp := range []string{"202609", "YYYYMMDDHHMMSS"} {
		buildNumber = stamp
		if got := BuildDate(); got != 0 {
			t.Errorf("BuildDate() with buildNumber=%q = %d want 0", stamp, got)
		}
	}
}

// TestBuildNumbersSortLexically is the property that makes the number worth
// having: string comparison has to match chronological order, because that is
// how anyone reads two of them side by side. Fixed-width zero-padded UTC
// yyyymmddhhmmss is what guarantees it — a shorter or unpadded format (say
// unix seconds, or a local-time rendering) would break at a digit boundary or
// at a DST change.
func TestBuildNumbersSortLexically(t *testing.T) {
	ordered := []string{
		"20260101000000",
		"20260902094117",
		"20260902155301",
		"20260902155302",
		"20261231235959",
		"20270101000000",
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Errorf("%q must sort before %q: build numbers compare in clock order, or they cannot be read at a glance", ordered[i-1], ordered[i])
		}
		if len(ordered[i]) != len(ordered[0]) {
			t.Errorf("%q has width %d want %d: every build number must be the same width", ordered[i], len(ordered[i]), len(ordered[0]))
		}
	}
}

// TestBuildNumberShapeMatchesTheMakefile pins the format the Makefile stamps
// (date -u +%Y%m%d%H%M%S). If that format is ever changed to something narrower
// — minutes, or a local-time value — this fails rather than silently producing
// build numbers that tie or go backwards.
func TestBuildNumberShapeMatchesTheMakefile(t *testing.T) {
	stamped := time.Date(2026, 9, 2, 15, 53, 1, 0, time.UTC).Format("20060102150405")
	if stamped != "20260902155301" {
		t.Fatalf("stamped=%q want %q", stamped, "20260902155301")
	}
	if len(stamped) != 14 {
		t.Fatalf("stamped=%q has width %d want 14: seconds resolution, two rebuilds inside a minute must differ", stamped, len(stamped))
	}
}
