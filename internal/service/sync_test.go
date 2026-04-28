package service

import (
	"context"
	"sort"
	"testing"

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
