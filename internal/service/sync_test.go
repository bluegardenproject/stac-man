package service

import (
	"context"
	"sort"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/gh"
	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// graphFixture builds an in-memory store + stack graph with the given
// trunk, parent map, and PR map. It's the unit-test seam for the pure
// retargetPlan function: pass in any topology, assert the desired
// gh-edit calls without touching git or gh.
func graphFixture(t *testing.T, trunk string, parents map[string]string, prs map[string]int) *stack.Graph {
	t.Helper()
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: trunk, Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	for branch, parent := range parents {
		meta := store.BranchMeta{Parent: parent, ParentSHA: "sha-" + parent}
		if pr, ok := prs[branch]; ok {
			meta.PR = pr
		}
		if err := mem.SetBranch(ctx, branch, meta); err != nil {
			t.Fatalf("SetBranch %s: %v", branch, err)
		}
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	return g
}

// sortedRetargets makes the planner's output order-independent for
// equality assertions — callers shouldn't depend on map iteration
// order from g.ChildrenOf.
func sortedRetargets(in []retargetEntry) []retargetEntry {
	out := append([]retargetEntry(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Branch < out[j].Branch })
	return out
}

func TestRetargetPlanIncludesChildrenWithPRs(t *testing.T) {
	// main → mergedBranch → child (PR #42)
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
			"child":        "mergedBranch",
		},
		map[string]int{
			"mergedBranch": 1,
			"child":        42,
		},
	)
	got := sortedRetargets(retargetPlan(g, "mergedBranch", "main"))
	want := []retargetEntry{{Branch: "child", PR: 42, NewBase: "main"}}
	if !equalRetargets(got, want) {
		t.Fatalf("retargetPlan = %#v, want %#v", got, want)
	}
}

func TestRetargetPlanSkipsChildrenWithoutPR(t *testing.T) {
	// child has no PR → nothing to retarget on GitHub.
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
			"child":        "mergedBranch",
		},
		map[string]int{
			"mergedBranch": 1,
			// child intentionally omitted
		},
	)
	got := retargetPlan(g, "mergedBranch", "main")
	if len(got) != 0 {
		t.Fatalf("retargetPlan = %#v, want empty", got)
	}
}

func TestRetargetPlanSkipsGrandchildren(t *testing.T) {
	// We retarget direct children only. Grandchildren keep their
	// existing parent PR base since their direct parent didn't merge.
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
			"child":        "mergedBranch",
			"grandchild":   "child",
		},
		map[string]int{
			"mergedBranch": 1,
			"child":        2,
			"grandchild":   3,
		},
	)
	got := sortedRetargets(retargetPlan(g, "mergedBranch", "main"))
	want := []retargetEntry{{Branch: "child", PR: 2, NewBase: "main"}}
	if !equalRetargets(got, want) {
		t.Fatalf("retargetPlan = %#v, want %#v", got, want)
	}
}

func TestRetargetPlanForkChildrenAllRetargeted(t *testing.T) {
	// main → mergedBranch → {a, b}. Both should retarget to main.
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
			"a":            "mergedBranch",
			"b":            "mergedBranch",
		},
		map[string]int{
			"mergedBranch": 1,
			"a":            10,
			"b":            11,
		},
	)
	got := sortedRetargets(retargetPlan(g, "mergedBranch", "main"))
	want := []retargetEntry{
		{Branch: "a", PR: 10, NewBase: "main"},
		{Branch: "b", PR: 11, NewBase: "main"},
	}
	if !equalRetargets(got, want) {
		t.Fatalf("retargetPlan = %#v, want %#v", got, want)
	}
}

func TestRetargetPlanForkMixedPRState(t *testing.T) {
	// main → mergedBranch → {withPR, withoutPR}. Only withPR is in
	// the plan; withoutPR has nothing on GitHub to retarget.
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
			"withPR":       "mergedBranch",
			"withoutPR":    "mergedBranch",
		},
		map[string]int{
			"mergedBranch": 1,
			"withPR":       42,
		},
	)
	got := sortedRetargets(retargetPlan(g, "mergedBranch", "main"))
	want := []retargetEntry{{Branch: "withPR", PR: 42, NewBase: "main"}}
	if !equalRetargets(got, want) {
		t.Fatalf("retargetPlan = %#v, want %#v", got, want)
	}
}

func TestRetargetPlanNoChildrenNoPlan(t *testing.T) {
	// Leaf merged branch — nothing to retarget.
	g := graphFixture(t, "main",
		map[string]string{
			"mergedBranch": "main",
		},
		map[string]int{
			"mergedBranch": 1,
		},
	)
	got := retargetPlan(g, "mergedBranch", "main")
	if len(got) != 0 {
		t.Fatalf("retargetPlan = %#v, want empty", got)
	}
}

// retargetPlan re-parents children onto the merged branch's parent,
// not necessarily trunk. Lock that in: when the merged branch is
// mid-stack, the new base for its child PR is the grandparent
// branch's name, not "main".
func TestRetargetPlanMidStackTargetsGrandparent(t *testing.T) {
	// main → top → mergedBranch → child
	g := graphFixture(t, "main",
		map[string]string{
			"top":          "main",
			"mergedBranch": "top",
			"child":        "mergedBranch",
		},
		map[string]int{
			"top":          1,
			"mergedBranch": 2,
			"child":        3,
		},
	)
	got := sortedRetargets(retargetPlan(g, "mergedBranch", "top"))
	want := []retargetEntry{{Branch: "child", PR: 3, NewBase: "top"}}
	if !equalRetargets(got, want) {
		t.Fatalf("retargetPlan = %#v, want %#v", got, want)
	}
}

func equalRetargets(a, b []retargetEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// --- mergedByPRState ---------------------------------------------------

func TestMergedByPRStateFlagsMergedPRs(t *testing.T) {
	g := graphFixture(t, "main",
		map[string]string{
			"merged-by-squash": "main",
			"still-open":       "main",
		},
		map[string]int{
			"merged-by-squash": 7,
			"still-open":       8,
		},
	)
	prs := map[string]gh.PR{
		"merged-by-squash": {Number: 7, State: gh.PRStateMerged},
		"still-open":       {Number: 8, State: gh.PRStateOpen},
	}
	got := mergedByPRState(g, prs, nil)
	if len(got) != 1 || got[0] != "merged-by-squash" {
		t.Fatalf("mergedByPRState = %v, want [merged-by-squash]", got)
	}
}

func TestMergedByPRStateIgnoresClosedAndDraft(t *testing.T) {
	// Closed (not merged) and any other non-MERGED state must not
	// trigger deletion. We deliberately don't conflate "closed
	// without merging" with "merged" — closed-without-merging
	// branches can still be revived by reopening the PR.
	g := graphFixture(t, "main",
		map[string]string{
			"closed": "main",
			"open":   "main",
		},
		map[string]int{
			"closed": 1,
			"open":   2,
		},
	)
	prs := map[string]gh.PR{
		"closed": {Number: 1, State: gh.PRStateClosed},
		"open":   {Number: 2, State: gh.PRStateOpen, IsDraft: true},
	}
	got := mergedByPRState(g, prs, nil)
	if len(got) != 0 {
		t.Fatalf("mergedByPRState = %v, want empty", got)
	}
}

func TestMergedByPRStateHonorsExclude(t *testing.T) {
	// `excluded` is the seam Sync uses to skip branches already
	// flagged by history-based detection — same branch must not
	// appear in both lists.
	g := graphFixture(t, "main",
		map[string]string{"already-merged": "main"},
		map[string]int{"already-merged": 9},
	)
	prs := map[string]gh.PR{
		"already-merged": {Number: 9, State: gh.PRStateMerged},
	}
	exclude := map[string]bool{"already-merged": true}
	got := mergedByPRState(g, prs, exclude)
	if len(got) != 0 {
		t.Fatalf("mergedByPRState = %v, want empty (excluded)", got)
	}
}

func TestMergedByPRStateSkipsBranchesWithoutLocalPR(t *testing.T) {
	// A tracked branch with PR == 0 (never submitted) can't be
	// "merged on GitHub" — there's nothing to look up. We avoid
	// even visiting it so a stack with no submissions doesn't pay
	// for a gh round-trip per branch.
	g := graphFixture(t, "main",
		map[string]string{"never-submitted": "main"},
		map[string]int{},
	)
	// Even if gh somehow returned data for the branch (e.g., a
	// stale PR re-mapped to a different branch), we still ignore
	// it because the local record has PR == 0.
	prs := map[string]gh.PR{
		"never-submitted": {Number: 99, State: gh.PRStateMerged},
	}
	got := mergedByPRState(g, prs, nil)
	if len(got) != 0 {
		t.Fatalf("mergedByPRState = %v, want empty (no local PR record)", got)
	}
}

func TestMergedByPRStateNoNetworkData(t *testing.T) {
	// gh fetch failed → empty map. Don't synthesise merges.
	g := graphFixture(t, "main",
		map[string]string{"submitted": "main"},
		map[string]int{"submitted": 5},
	)
	got := mergedByPRState(g, map[string]gh.PR{}, nil)
	if len(got) != 0 {
		t.Fatalf("mergedByPRState = %v, want empty", got)
	}
}

// --- sortMergedByDepth -------------------------------------------------

func TestSortMergedByDepthDescendantsFirst(t *testing.T) {
	// main → a → b → c. If all three are merged we must delete in
	// c, b, a order so re-parenting never tries to move a child
	// onto a parent that's already gone.
	g := graphFixture(t, "main",
		map[string]string{
			"a": "main",
			"b": "a",
			"c": "b",
		},
		nil,
	)
	got := sortMergedByDepth(g, []string{"a", "b", "c"}, "main")
	want := []string{"c", "b", "a"}
	if !equalStrings(got, want) {
		t.Fatalf("sortMergedByDepth = %v, want %v", got, want)
	}
}

func TestSortMergedByDepthForkPreservesRelativeOrder(t *testing.T) {
	// main → mid → {leafA, leafB}. mid is depth 1; leaves are
	// depth 2. Both leaves must come before mid.
	g := graphFixture(t, "main",
		map[string]string{
			"mid":   "main",
			"leafA": "mid",
			"leafB": "mid",
		},
		nil,
	)
	got := sortMergedByDepth(g, []string{"mid", "leafA", "leafB"}, "main")
	if got[len(got)-1] != "mid" {
		t.Fatalf("sortMergedByDepth = %v, want mid last", got)
	}
}

// --- untrackAndDelete: SHA preservation --------------------------------

// untrackAndDelete must NOT pre-update children's recorded ParentSHA
// when re-parenting. Setting it to the new parent's tip would defeat
// the restack engine's mismatch check (recorded SHA == live tip →
// no rebase) and silently leave the child's history pointing at
// commits that no longer exist on trunk after a squash- or
// merge-commit landing. Same anti-pattern as B5.
func TestUntrackAndDeletePreservesChildParentSHA(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	// main → mergedBranch → child, where child's recorded ParentSHA
	// points at mergedBranch's old tip (sha-old). Sync re-parents
	// child onto main and we want sha-old to stay put.
	if err := mem.SetBranch(ctx, "mergedBranch", store.BranchMeta{Parent: "main", ParentSHA: "sha-main-old"}); err != nil {
		t.Fatalf("SetBranch mergedBranch: %v", err)
	}
	if err := mem.SetBranch(ctx, "child", store.BranchMeta{Parent: "mergedBranch", ParentSHA: "sha-old"}); err != nil {
		t.Fatalf("SetBranch child: %v", err)
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	// scriptedRunner returns sha-new for any rev-parse — i.e., main's
	// current tip differs from the child's recorded sha-old. The
	// preservation check is meaningful precisely because the SHAs
	// would diverge if untrackAndDelete pre-updated.
	runner := scriptedRunner{byCmd: map[string]string{
		"rev-parse": "sha-new",
	}}
	svc := &Service{G: git.NewWithRunner(runner), Store: mem}

	if _, err := svc.untrackAndDelete(ctx, g, "mergedBranch", "main"); err != nil {
		t.Fatalf("untrackAndDelete: %v", err)
	}
	got, _, err := mem.GetBranch(ctx, "child")
	if err != nil {
		t.Fatalf("GetBranch child: %v", err)
	}
	if got.Parent != "main" {
		t.Fatalf("child Parent = %q, want main", got.Parent)
	}
	if got.ParentSHA != "sha-old" {
		t.Fatalf("child ParentSHA = %q, want sha-old (preserved); pre-updating it defeats the restack engine's mismatch check", got.ParentSHA)
	}
}

func TestUntrackAndDeleteUnsetsMergedBranch(t *testing.T) {
	// Companion to the SHA-preservation test: the merged branch's
	// own metadata must be removed so a follow-up `sm log` doesn't
	// keep showing it.
	ctx := context.Background()
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "mergedBranch", store.BranchMeta{Parent: "main", ParentSHA: "sha-main"}); err != nil {
		t.Fatalf("SetBranch: %v", err)
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	runner := scriptedRunner{byCmd: map[string]string{"rev-parse": "sha-main"}}
	svc := &Service{G: git.NewWithRunner(runner), Store: mem}

	if _, err := svc.untrackAndDelete(ctx, g, "mergedBranch", "main"); err != nil {
		t.Fatalf("untrackAndDelete: %v", err)
	}
	if _, ok, _ := mem.GetBranch(ctx, "mergedBranch"); ok {
		t.Fatal("mergedBranch metadata still tracked after untrackAndDelete")
	}
}
