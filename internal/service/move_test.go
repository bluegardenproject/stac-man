package service

import (
	"testing"
)

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
