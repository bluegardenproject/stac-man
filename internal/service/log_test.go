package service

import (
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/gh"
)

// TestRenderCheckDotHidesUnknown pins the "no checks configured" rule:
// `sm log` must not emit a permanent grey dot for repos that simply
// don't run CI on PRs. ChecksNone is treated identically to a missing
// rollup so the renderer collapses the empty cell.
func TestRenderCheckDotHidesUnknown(t *testing.T) {
	if got := renderCheckDot(gh.ChecksNone); got != "" {
		t.Fatalf("renderCheckDot(ChecksNone) = %q, want empty", got)
	}
}

// TestRenderCheckDotShowsAllNonEmptyStates locks down the three
// state→glyph mappings the user actually sees. The exact ANSI
// styling isn't asserted; we only care that each rollup produces a
// non-empty string with the bullet character so the layout column
// stays consistent across states.
func TestRenderCheckDotShowsAllNonEmptyStates(t *testing.T) {
	cases := []gh.CheckRollup{gh.ChecksPass, gh.ChecksPending, gh.ChecksFail}
	for _, c := range cases {
		got := renderCheckDot(c)
		if got == "" {
			t.Fatalf("renderCheckDot(%s) = empty, want a rendered bullet", c)
		}
		if !strings.Contains(got, "●") {
			t.Fatalf("renderCheckDot(%s) = %q, want bullet glyph", c, got)
		}
	}
}

// TestRenderMergeGlyphAllStates pins the three-state mergeability
// vocabulary the roadmap calls out: ✓ for MERGEABLE, ⚠ for
// CONFLICTING, ? for UNKNOWN. The empty default catches an
// accidentally added Mergeability value that the renderer has no
// glyph for — a future field would otherwise silently render blank.
func TestRenderMergeGlyphAllStates(t *testing.T) {
	cases := []struct {
		in   gh.Mergeability
		want string
	}{
		{gh.MergeMergeable, "✓"},
		{gh.MergeConflicting, "⚠"},
		{gh.MergeUnknown, "?"},
	}
	for _, tc := range cases {
		got := renderMergeGlyph(tc.in)
		if !strings.Contains(got, tc.want) {
			t.Fatalf("renderMergeGlyph(%s) = %q, want it to contain %q", tc.in, got, tc.want)
		}
	}
}

// TestRenderMergeGlyphRejectsUnrecognized guards against silent
// behavior change if a new Mergeability constant lands without a
// corresponding renderer arm.
func TestRenderMergeGlyphRejectsUnrecognized(t *testing.T) {
	if got := renderMergeGlyph("BOGUS"); got != "" {
		t.Fatalf("renderMergeGlyph(\"BOGUS\") = %q, want empty", got)
	}
}
