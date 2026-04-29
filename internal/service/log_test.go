package service

import (
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/gh"
)

// TestRenderCheckBadgeHidesNone pins the "no checks configured" rule:
// `sm log` must not emit a permanent neutral CI badge for repos that
// simply don't run CI on PRs. ChecksNone collapses the column.
func TestRenderCheckBadgeHidesNone(t *testing.T) {
	if got := renderCheckBadge(gh.ChecksNone); got != "" {
		t.Fatalf("renderCheckBadge(ChecksNone) = %q, want empty", got)
	}
}

// TestRenderCheckBadgeShowsAllNonEmptyStates locks down that every
// non-empty rollup produces a visible "CI" badge — the colour
// changes between pass / pending / fail but the label stays
// constant so the column lines up across rows.
func TestRenderCheckBadgeShowsAllNonEmptyStates(t *testing.T) {
	for _, c := range []gh.CheckRollup{gh.ChecksPass, gh.ChecksPending, gh.ChecksFail} {
		got := renderCheckBadge(c)
		if got == "" {
			t.Fatalf("renderCheckBadge(%s) = empty, want a CI badge", c)
		}
		if !strings.Contains(got, "CI") {
			t.Fatalf("renderCheckBadge(%s) = %q, want it to contain %q", c, got, "CI")
		}
	}
}

// TestRenderMergeBadgeAllStates pins the two-tone mergeability
// vocabulary the user asked for: green "ready" when MERGEABLE,
// orange "conflict" when CONFLICTING. UNKNOWN suppresses the badge
// so the column doesn't show a noisy placeholder while GitHub is
// still computing the merge state.
func TestRenderMergeBadgeAllStates(t *testing.T) {
	cases := []struct {
		in        gh.Mergeability
		wantLabel string
	}{
		{gh.MergeMergeable, "ready"},
		{gh.MergeConflicting, "conflict"},
	}
	for _, tc := range cases {
		got := renderMergeBadge(tc.in)
		if !strings.Contains(got, tc.wantLabel) {
			t.Fatalf("renderMergeBadge(%s) = %q, want it to contain %q", tc.in, got, tc.wantLabel)
		}
	}
	if got := renderMergeBadge(gh.MergeUnknown); got != "" {
		t.Fatalf("renderMergeBadge(MergeUnknown) = %q, want empty", got)
	}
}

// TestRenderMergeBadgeRejectsUnrecognized guards against silent
// behavior change if a new Mergeability constant lands without a
// corresponding renderer arm.
func TestRenderMergeBadgeRejectsUnrecognized(t *testing.T) {
	if got := renderMergeBadge("BOGUS"); got != "" {
		t.Fatalf("renderMergeBadge(\"BOGUS\") = %q, want empty", got)
	}
}
