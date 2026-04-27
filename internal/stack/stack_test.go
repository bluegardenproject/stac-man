package stack

import (
	"context"
	"errors"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// buildGraph constructs an in-memory graph with the given trunk and
// parent map. It's a helper so each test can express the topology it
// cares about in one place.
func buildGraph(t *testing.T, trunk string, parents map[string]string) *Graph {
	t.Helper()
	s := memory.New()
	ctx := context.Background()
	if err := s.SetRepo(ctx, store.RepoMeta{Trunk: trunk, Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	for branch, parent := range parents {
		if err := s.SetBranch(ctx, branch, store.BranchMeta{Parent: parent, ParentSHA: "sha-" + parent}); err != nil {
			t.Fatalf("SetBranch %s: %v", branch, err)
		}
	}
	g, err := Load(ctx, s)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

// example topology used by several tests:
//
//	main
//	 ├── feat-a
//	 │    ├── feat-b
//	 │    └── feat-c
//	 │         └── feat-d
//	 └── feat-e
func exampleParents() map[string]string {
	return map[string]string{
		"feat-a": "main",
		"feat-b": "feat-a",
		"feat-c": "feat-a",
		"feat-d": "feat-c",
		"feat-e": "main",
	}
}

func TestRoots(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	roots := g.Roots()
	if len(roots) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(roots))
	}
	if roots[0].Name != "feat-a" || roots[1].Name != "feat-e" {
		t.Fatalf("unexpected roots: %v", names(roots))
	}
}

func TestChildrenOf(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	got := names(g.ChildrenOf("feat-a"))
	want := []string{"feat-b", "feat-c"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAncestors(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	got := names(g.Ancestors("feat-d"))
	want := []string{"feat-c", "feat-a"}
	if !equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestDescendantsAndTopoOrder(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	desc := names(g.Descendants("feat-a"))
	// DFS, alpha order at each level.
	want := []string{"feat-b", "feat-c", "feat-d"}
	if !equal(desc, want) {
		t.Fatalf("descendants got %v, want %v", desc, want)
	}

	topo := names(g.TopoOrderFrom("feat-a"))
	wantTopo := []string{"feat-a", "feat-b", "feat-c", "feat-d"}
	if !equal(topo, wantTopo) {
		t.Fatalf("topo got %v, want %v", topo, wantTopo)
	}
}

func TestValidateClean(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	local := []string{"main", "feat-a", "feat-b", "feat-c", "feat-d", "feat-e"}
	if errs := g.Validate(local); len(errs) != 0 {
		t.Fatalf("expected clean graph, got %v", errs)
	}
}

func TestValidateMissingLocalRef(t *testing.T) {
	g := buildGraph(t, "main", exampleParents())
	// feat-d is tracked but no longer present locally.
	local := []string{"main", "feat-a", "feat-b", "feat-c", "feat-e"}
	errs := g.Validate(local)
	if len(errs) == 0 {
		t.Fatalf("expected an error for missing local ref")
	}
}

func TestValidateUntrackedParent(t *testing.T) {
	// feat-x's parent feat-y was never tracked.
	g := buildGraph(t, "main", map[string]string{
		"feat-x": "feat-y",
	})
	errs := g.Validate([]string{"main", "feat-x"})
	if len(errs) == 0 {
		t.Fatalf("expected an error for untracked parent")
	}
}

func TestValidateCycle(t *testing.T) {
	// Build a graph by writing to the store directly to bypass any
	// guarding in higher layers — Validate is the safety net.
	s := memory.New()
	ctx := context.Background()
	_ = s.SetRepo(ctx, store.RepoMeta{Trunk: "main"})
	_ = s.SetBranch(ctx, "a", store.BranchMeta{Parent: "b"})
	_ = s.SetBranch(ctx, "b", store.BranchMeta{Parent: "a"})
	g, err := Load(ctx, s)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	errs := g.Validate([]string{"main", "a", "b"})
	if len(errs) == 0 {
		t.Fatalf("expected cycle error, got none")
	}
	cycleSeen := false
	for _, e := range errs {
		if errors.Is(e, ErrCycle) {
			cycleSeen = true
			break
		}
	}
	if !cycleSeen {
		t.Fatalf("expected ErrCycle in errors, got %v", errs)
	}
}

func TestEmptyGraph(t *testing.T) {
	s := memory.New()
	_ = s.SetRepo(context.Background(), store.RepoMeta{Trunk: "main"})
	g, err := Load(context.Background(), s)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(g.Branches()) != 0 {
		t.Fatalf("expected empty graph")
	}
	if len(g.Roots()) != 0 {
		t.Fatalf("expected no roots")
	}
}

func names(bs []Branch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}

func equal(a, b []string) bool {
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
