package service

import (
	"context"
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
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

// loadLogGraph builds a small non-linear stack used by the buildLogTree
// and renderLogTree tests:
//
//	main
//	├─ feat-a
//	│  └─ feat-b
//	└─ feat-c
//
// Two roots ensures we exercise the multi-root path; one parent with
// a child ensures we exercise the recursive walk.
func loadLogGraph(t *testing.T) *stack.Graph {
	t.Helper()
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	for _, b := range []struct {
		name, parent, parentSHA string
		pr                      int
	}{
		{"feat-a", "main", "sha-main", 10},
		{"feat-b", "feat-a", "sha-a", 11},
		{"feat-c", "main", "sha-main", 0},
	} {
		if err := mem.SetBranch(ctx, b.name, store.BranchMeta{Parent: b.parent, ParentSHA: b.parentSHA, PR: b.pr}); err != nil {
			t.Fatalf("SetBranch %s: %v", b.name, err)
		}
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	return g
}

// TestBuildLogTreePinsShape locks down the structure produced for the
// fixture graph: two roots in deterministic order, feat-a carrying
// one child feat-b, feat-c carrying no children. This is the contract
// the renderer (and step 2's JSON / porcelain formatters) reads from.
func TestBuildLogTreePinsShape(t *testing.T) {
	g := loadLogGraph(t)
	roots := buildLogTree(g, nil, nil, nil)

	if len(roots) != 2 {
		t.Fatalf("roots: got %d, want 2", len(roots))
	}
	rootNames := []string{roots[0].Branch.Name, roots[1].Branch.Name}
	if !containsString(rootNames, "feat-a") || !containsString(rootNames, "feat-c") {
		t.Fatalf("root names = %v, want feat-a and feat-c", rootNames)
	}

	var aNode *logBranchNode
	for _, r := range roots {
		if r.Branch.Name == "feat-a" {
			aNode = r
		}
	}
	if aNode == nil {
		t.Fatalf("feat-a not present at root level")
	}
	if len(aNode.Children) != 1 || aNode.Children[0].Branch.Name != "feat-b" {
		t.Fatalf("feat-a children = %v, want [feat-b]", branchNamesOfNodes(aNode.Children))
	}
}

// TestBuildLogTreeWiresPRAndStatus pins that the PR / Status pointer
// fields are populated when the input maps carry an entry for the
// branch and stay nil otherwise. This is the seam the JSON formatter
// will rely on to distinguish "we know there's no PR" from "we
// haven't fetched PRs at all" — the existing renderer does the same.
func TestBuildLogTreeWiresPRAndStatus(t *testing.T) {
	g := loadLogGraph(t)
	prMap := map[string]gh.PR{
		"feat-a": {Number: 10, State: gh.PRStateOpen},
	}
	statusMap := map[string]gh.PRStatus{
		"feat-a": {Number: 10, Checks: gh.ChecksPass, Mergeable: gh.MergeMergeable, State: gh.PRStateOpen},
	}
	roots := buildLogTree(g, prMap, statusMap, map[string]bool{"feat-b": true})

	for _, r := range roots {
		switch r.Branch.Name {
		case "feat-a":
			if r.PR == nil || r.PR.Number != 10 {
				t.Fatalf("feat-a PR = %+v, want number=10", r.PR)
			}
			if r.Status == nil || r.Status.Checks != gh.ChecksPass {
				t.Fatalf("feat-a Status = %+v, want Checks=Pass", r.Status)
			}
			if r.NeedsRestack {
				t.Fatalf("feat-a should not be flagged needsRestack — only feat-b is")
			}
			if len(r.Children) != 1 {
				t.Fatalf("feat-a should still carry feat-b as a child")
			}
			if !r.Children[0].NeedsRestack {
				t.Fatalf("feat-b NeedsRestack: got false, want true (was set in needsRestack map)")
			}
		case "feat-c":
			if r.PR != nil || r.Status != nil {
				t.Fatalf("feat-c had no entry in maps; PR=%+v Status=%+v should both be nil", r.PR, r.Status)
			}
		}
	}
}

// TestBuildLogTreeEmptyGraph guards the no-tracked-branches edge case:
// roots come back as an empty (non-nil) slice so the renderer can
// special-case it without a nil check. This shape matches the
// pre-refactor behaviour where Log emitted the "(no tracked branches…)"
// hint when g.Roots() was empty.
func TestBuildLogTreeEmptyGraph(t *testing.T) {
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	roots := buildLogTree(g, nil, nil, nil)
	if roots == nil {
		t.Fatalf("roots = nil, want empty non-nil slice")
	}
	if len(roots) != 0 {
		t.Fatalf("roots = %v, want empty", branchNamesOfNodes(roots))
	}
}

// TestRenderLogTreeContainsExpectedTokens characterises the rendered
// output so the step-2 feature work doesn't accidentally drift the
// shape. The exact ANSI bytes vary with terminal width / TTY; we
// check the textual skeleton (banner, trunk header, every branch
// name with its connector, the current marker, the PR pill) which
// is what users actually read.
func TestRenderLogTreeContainsExpectedTokens(t *testing.T) {
	d := &logData{
		Trunk:   "main",
		Current: "feat-b",
		Roots: []*logBranchNode{
			{
				Branch: stack.Branch{Name: "feat-a", Parent: "main", PR: 10},
				PR:     &gh.PR{Number: 10, State: gh.PRStateOpen},
				Children: []*logBranchNode{
					{
						Branch: stack.Branch{Name: "feat-b", Parent: "feat-a", PR: 11},
						PR:     &gh.PR{Number: 11, State: gh.PRStateOpen},
					},
				},
			},
			{
				Branch:       stack.Branch{Name: "feat-c", Parent: "main"},
				NeedsRestack: true,
			},
		},
	}
	out := renderLogTree(d, LogOptions{IncludePRStatus: true, IncludeChecks: true, IncludeMergeStatus: true})

	for _, want := range []string{
		"stac-man",        // banner
		"main",            // trunk header
		"feat-a",          // first root
		"feat-b",          // descendant
		"feat-c",          // second root
		"← current",       // current marker on feat-b
		"#10 open",        // PR pill on feat-a
		"#11 open",        // PR pill on feat-b
		"(needs restack)", // feat-c flagged
		"└─",              // last-child connector
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("renderLogTree output missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// TestRenderLogTreeEmptyGraphHint pins the "(no tracked branches…)"
// hint, which is the user's onboarding signal in a freshly cloned
// repo. The pre-refactor Log printed it whenever roots was empty;
// the new renderer must keep that behaviour.
func TestRenderLogTreeEmptyGraphHint(t *testing.T) {
	d := &logData{Trunk: "main"}
	out := renderLogTree(d, LogOptions{})
	if !strings.Contains(out, "no tracked branches yet") {
		t.Fatalf("empty-graph hint missing\n--- got ---\n%s", out)
	}
}

func containsString(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func branchNamesOfNodes(ns []*logBranchNode) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Branch.Name)
	}
	return out
}
