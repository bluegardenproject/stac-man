package service

import (
	"context"
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// buildTestGraph wires up a memory-backed graph for navigation tests.
//
//	main
//	 ├── feat-a
//	 │    ├── feat-b
//	 │    └── feat-c
//	 │         └── feat-d
//	 └── feat-e
func buildTestGraph(t *testing.T) *stack.Graph {
	t.Helper()
	s := memory.New()
	ctx := context.Background()
	if err := s.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	parents := map[string]string{
		"feat-a": "main",
		"feat-b": "feat-a",
		"feat-c": "feat-a",
		"feat-d": "feat-c",
		"feat-e": "main",
	}
	for branch, parent := range parents {
		if err := s.SetBranch(ctx, branch, store.BranchMeta{Parent: parent}); err != nil {
			t.Fatalf("SetBranch %s: %v", branch, err)
		}
	}
	g, err := stack.Load(ctx, s)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return g
}

func TestResolveDown(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-d", DirDown, false)
	if err != nil {
		t.Fatalf("DirDown: %v", err)
	}
	if got != "feat-c" {
		t.Fatalf("got %q, want feat-c", got)
	}
}

func TestResolveDownFromRootGoesToTrunk(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-a", DirDown, false)
	if err != nil {
		t.Fatalf("DirDown: %v", err)
	}
	if got != "main" {
		t.Fatalf("got %q, want main", got)
	}
}

func TestResolveDownFromTrunkErrors(t *testing.T) {
	g := buildTestGraph(t)
	_, err := resolveDirection(g, "main", "main", DirDown, false)
	if err == nil {
		t.Fatalf("expected error when calling DirDown on trunk")
	}
}

func TestResolveUpForkRequiresFlag(t *testing.T) {
	g := buildTestGraph(t)
	// feat-a has two children — must error without preferAlpha.
	_, err := resolveDirection(g, "main", "feat-a", DirUp, false)
	if err == nil || !strings.Contains(err.Error(), "multiple children") {
		t.Fatalf("expected multiple-children error, got %v", err)
	}
	got, err := resolveDirection(g, "main", "feat-a", DirUp, true)
	if err != nil {
		t.Fatalf("preferAlpha: %v", err)
	}
	if got != "feat-b" {
		t.Fatalf("alpha-first should be feat-b, got %q", got)
	}
}

func TestResolveUpLinearChain(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-c", DirUp, false)
	if err != nil {
		t.Fatalf("DirUp: %v", err)
	}
	if got != "feat-d" {
		t.Fatalf("got %q, want feat-d", got)
	}
}

func TestResolveUpLeafErrors(t *testing.T) {
	g := buildTestGraph(t)
	_, err := resolveDirection(g, "main", "feat-d", DirUp, false)
	if err == nil {
		t.Fatalf("expected error at leaf")
	}
}

func TestResolveBottom(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-d", DirBottom, false)
	if err != nil {
		t.Fatalf("DirBottom: %v", err)
	}
	if got != "feat-a" {
		t.Fatalf("got %q, want feat-a", got)
	}
}

func TestResolveBottomFromRoot(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-a", DirBottom, false)
	if err != nil {
		t.Fatalf("DirBottom: %v", err)
	}
	if got != "feat-a" {
		t.Fatalf("got %q, want feat-a (already at bottom)", got)
	}
}

func TestResolveTopLinear(t *testing.T) {
	g := buildTestGraph(t)
	got, err := resolveDirection(g, "main", "feat-c", DirTop, false)
	if err != nil {
		t.Fatalf("DirTop: %v", err)
	}
	if got != "feat-d" {
		t.Fatalf("got %q, want feat-d", got)
	}
}

func TestResolveTopForkRequiresFlag(t *testing.T) {
	g := buildTestGraph(t)
	_, err := resolveDirection(g, "main", "feat-a", DirTop, false)
	if err == nil || !strings.Contains(err.Error(), "fork") {
		t.Fatalf("expected fork error, got %v", err)
	}
	got, err := resolveDirection(g, "main", "feat-a", DirTop, true)
	if err != nil {
		t.Fatalf("DirTop with preferAlpha: %v", err)
	}
	// alpha-first child of feat-a is feat-b (no further children → leaf)
	if got != "feat-b" {
		t.Fatalf("got %q, want feat-b", got)
	}
}
