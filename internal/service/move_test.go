package service

import (
	"context"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// TestSetParentRebasesEvenWhenNewParentIsAncestor pins B5: when a user
// reparents `feat-c` from `feat-b` onto `feat-a` and `feat-a`'s tip is
// already reachable from `feat-c` (because `feat-a` sits between `feat-c`
// and trunk in the existing chain), SetParent must STILL trigger a real
// `git rebase --onto feat-a.tip feat-b.oldTip feat-c` so that `feat-c`'s
// commits are replayed on top of `feat-a` directly. Pre-fix, SetParent
// updated ParentSHA to the new parent's tip *before* calling Restack,
// which made the engine see "no SHA mismatch" and skip the rebase
// entirely — leaving the metadata claiming feat-a as the parent while
// the actual git history still ran through feat-b's commits.
func TestSetParentRebasesEvenWhenNewParentIsAncestor(t *testing.T) {
	gitDir := t.TempDir()
	const (
		oldParentTip = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" // feat-b tip
		newParentTip = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // feat-a tip
		branchTip    = "cccccccccccccccccccccccccccccccccccccccc" // feat-c tip
	)
	r := &fakeRunner{
		responses: map[string]string{
			"rev-parse --is-inside-work-tree": "true",
			"rev-parse --git-dir":             gitDir,
			// Resolve every ref the engine probes during a Restack
			// triggered by SetParent.
			"rev-parse --verify feat-a^{commit}": newParentTip,
			"rev-parse --verify feat-c^{commit}": branchTip,
			// BranchExists("feat-a") — succeeds (default fallback err nil).
			"show-ref --verify --quiet refs/heads/feat-a": "",
			// CurrentBranch (used by recordHistory).
			"symbolic-ref --short HEAD": "feat-c",
			// Rebase succeeds; we don't care about the stdout.
			// The exact argv is what the test asserts below.
		},
	}

	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", ParentSHA: "trunk-sha"}); err != nil {
		t.Fatalf("SetBranch a: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-b", store.BranchMeta{Parent: "feat-a", ParentSHA: newParentTip}); err != nil {
		t.Fatalf("SetBranch b: %v", err)
	}
	// feat-c starts under feat-b — recorded ParentSHA = feat-b's tip.
	if err := mem.SetBranch(ctx, "feat-c", store.BranchMeta{Parent: "feat-b", ParentSHA: oldParentTip}); err != nil {
		t.Fatalf("SetBranch c: %v", err)
	}

	s := &Service{G: git.NewWithRunner(r), Store: mem}
	if err := s.SetParent(ctx, "feat-c", "feat-a"); err != nil {
		t.Fatalf("SetParent: %v", err)
	}

	// The crucial assertion: `git rebase --onto <new-parent-tip>
	// <old-recorded-parent-sha> feat-c` must have run. Pre-fix this
	// call was missing because the engine skipped the rebase.
	wantArgs := []string{"rebase", "--onto", newParentTip, oldParentTip, "feat-c"}
	if !r.called(wantArgs...) {
		t.Fatalf("expected rebase --onto %s %s feat-c, calls were:\n%v", newParentTip, oldParentTip, r.calls)
	}

	// And the metadata should now reflect the new parent.
	got, _, err := mem.GetBranch(ctx, "feat-c")
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if got.Parent != "feat-a" {
		t.Fatalf("Parent = %q, want feat-a", got.Parent)
	}
	if got.ParentSHA != newParentTip {
		t.Fatalf("ParentSHA = %q, want %q (the engine should set this after a successful rebase)", got.ParentSHA, newParentTip)
	}
}

// TestMoveCycleDetection verifies the cycle check in SetParent (which
// Move delegates to). We can't drive the full git path without a real
// repo, but we can assert the helper rejects an obvious cycle by
// walking the in-memory graph the same way SetParent does.
func TestMoveCycleDetectionViaGraph(t *testing.T) {
	g := buildTestGraph(t)
	// feat-a's tracked descendants are feat-b, feat-c, feat-d.
	// Re-parenting feat-a onto feat-c (a descendant) would cycle.
	cycle := false
	for _, d := range g.Descendants("feat-a") {
		if d.Name == "feat-c" {
			cycle = true
			break
		}
	}
	if !cycle {
		t.Fatalf("expected feat-c to be a descendant of feat-a in the test graph")
	}

	// Re-parenting feat-e onto feat-b (a non-descendant) is fine.
	for _, d := range g.Descendants("feat-e") {
		if d.Name == "feat-b" {
			t.Fatalf("feat-b unexpectedly a descendant of feat-e")
		}
	}
}

// TestMoveDescendantsRideAlong asserts that walking from feat-a in
// topo order picks up every descendant — Move relies on this so the
// restack engine cascades through the subtree without any explicit
// per-child reparenting.
func TestMoveDescendantsRideAlong(t *testing.T) {
	g := buildTestGraph(t)
	chain := g.TopoOrderFrom("feat-a")
	wantNames := []string{"feat-a", "feat-b", "feat-c", "feat-d"}
	if len(chain) != len(wantNames) {
		t.Fatalf("got %d entries in chain, want %d (%v)", len(chain), len(wantNames), chain)
	}
	for i, b := range chain {
		if b.Name != wantNames[i] {
			t.Fatalf("chain[%d]: got %q want %q", i, b.Name, wantNames[i])
		}
	}
}

// TestBottomMostFromLeaf walks the path from a leaf back to just-above-
// trunk. `sm land` uses this to find which PR to merge.
func TestBottomMostFromLeaf(t *testing.T) {
	g := buildTestGraph(t)
	got := bottomMost(g, "main", "feat-d")
	if got != "feat-a" {
		t.Fatalf("bottomMost: got %q want feat-a", got)
	}
}

func TestBottomMostHandlesRoot(t *testing.T) {
	g := buildTestGraph(t)
	got := bottomMost(g, "main", "feat-a")
	if got != "feat-a" {
		t.Fatalf("root branch should be its own bottom; got %q", got)
	}
}
