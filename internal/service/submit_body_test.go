package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// loadChainGraph builds a single linear stack:
//
//	main → feat-a → feat-b → feat-c
//
// shared by the chain-shape tests so a parent re-parent doesn't have
// to be re-encoded in every case.
func loadChainGraph(t *testing.T) *stack.Graph {
	t.Helper()
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	for _, b := range []struct{ name, parent string }{
		{"feat-a", "main"},
		{"feat-b", "feat-a"},
		{"feat-c", "feat-b"},
	} {
		if err := mem.SetBranch(ctx, b.name, store.BranchMeta{Parent: b.parent}); err != nil {
			t.Fatalf("SetBranch %s: %v", b.name, err)
		}
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	return g
}

// TestStackChainForBranchMiddle pins the canonical case the roadmap
// describes: a branch in the middle of a stack sees its ancestors
// (oldest first) followed by itself followed by its descendants.
// This is the trunk-toward-leaf order reviewers expect when the
// stack table is rendered top-to-bottom.
func TestStackChainForBranchMiddle(t *testing.T) {
	g := loadChainGraph(t)
	got := branchNameSlice(stackChainForBranch(g, "feat-b"))
	want := []string{"feat-a", "feat-b", "feat-c"}
	if !equalStrings(got, want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
}

// TestStackChainForBranchLeaf pins the leaf case: descendants are
// empty, ancestors fill the chain. This is the most common shape on
// a freshly-stacked PR.
func TestStackChainForBranchLeaf(t *testing.T) {
	g := loadChainGraph(t)
	got := branchNameSlice(stackChainForBranch(g, "feat-c"))
	want := []string{"feat-a", "feat-b", "feat-c"}
	if !equalStrings(got, want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
}

// TestStackChainForBranchRoot pins the root case: ancestors are
// empty (the parent is trunk, which is excluded from the chain),
// descendants follow.
func TestStackChainForBranchRoot(t *testing.T) {
	g := loadChainGraph(t)
	got := branchNameSlice(stackChainForBranch(g, "feat-a"))
	want := []string{"feat-a", "feat-b", "feat-c"}
	if !equalStrings(got, want) {
		t.Fatalf("chain = %v, want %v", got, want)
	}
}

// TestStackChainForBranchSkipsSiblings pins the per-branch path
// view: a sibling branch is NOT included in this branch's chain,
// because its presence in the PR table would only confuse a
// reviewer who's looking at the wrong subtree.
func TestStackChainForBranchSkipsSiblings(t *testing.T) {
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	for _, b := range []struct{ name, parent string }{
		{"feat-a", "main"},
		{"feat-b", "feat-a"}, // descendant of feat-a
		{"feat-c", "feat-a"}, // sibling of feat-b
	} {
		if err := mem.SetBranch(ctx, b.name, store.BranchMeta{Parent: b.parent}); err != nil {
			t.Fatalf("SetBranch %s: %v", b.name, err)
		}
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}

	got := branchNameSlice(stackChainForBranch(g, "feat-b"))
	want := []string{"feat-a", "feat-b"}
	if !equalStrings(got, want) {
		t.Fatalf("chain = %v, want %v (feat-c must not appear)", got, want)
	}
}

// TestRenderStackTableEmptyForLoneBranch pins the no-decoration
// rule: a branch sitting alone on trunk produces no stack table at
// all. We deliberately don't emit a "stack of 1" block — the
// reviewer learns nothing from it and the noise hurts every
// non-stacked PR.
func TestRenderStackTableEmptyForLoneBranch(t *testing.T) {
	chain := []stack.Branch{{Name: "feat-x"}}
	got := renderStackTable(chain, "feat-x", map[string]int{"feat-x": 7})
	if got != "" {
		t.Fatalf("renderStackTable on lone branch = %q, want empty", got)
	}
}

// TestRenderStackTableLinearChain locks down the markdown shape: a
// sentinel-fenced block, the bolded "← this PR" marker on the
// current row, and the trunk-toward-leaf ordering. We stop short
// of asserting whitespace exactly so the renderer can tweak the
// header line without breaking the contract.
func TestRenderStackTableLinearChain(t *testing.T) {
	chain := []stack.Branch{
		{Name: "feat-a"},
		{Name: "feat-b"},
		{Name: "feat-c"},
	}
	prs := map[string]int{"feat-a": 1, "feat-b": 2, "feat-c": 3}
	got := renderStackTable(chain, "feat-b", prs)

	for _, want := range []string{
		stackTableStart,
		stackTableEnd,
		"#1 feat-a",
		"**#2 feat-b ← this PR**",
		"#3 feat-c",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderStackTable missing %q in:\n%s", want, got)
		}
	}
	// "← this PR" must appear exactly once: bolding any other row
	// would mislead the reviewer of #1 / #3 into thinking they were
	// looking at the right PR.
	if n := strings.Count(got, "← this PR"); n != 1 {
		t.Fatalf("expected exactly one current-row marker, got %d", n)
	}
	// Ordering: #1 line comes before #2 line comes before #3 line.
	idx := func(s string) int { return strings.Index(got, s) }
	if !(idx("feat-a") < idx("feat-b") && idx("feat-b") < idx("feat-c")) {
		t.Fatalf("rows out of order:\n%s", got)
	}
}

// TestRenderStackTableShowsBranchesWithoutPR pins the "unsubmitted
// gap" case: a branch whose parent has been pushed but whose own PR
// has not yet been opened still shows in the table — sans link, with
// a small marker. This is what tells the reviewer "this PR depends
// on something that isn't on GitHub yet".
func TestRenderStackTableShowsBranchesWithoutPR(t *testing.T) {
	chain := []stack.Branch{{Name: "feat-a"}, {Name: "feat-b"}}
	got := renderStackTable(chain, "feat-b", map[string]int{"feat-b": 2})
	if !strings.Contains(got, "feat-a") {
		t.Fatalf("expected unsubmitted parent in table:\n%s", got)
	}
	if !strings.Contains(got, "no PR") {
		t.Fatalf("expected a 'no PR' marker for the unsubmitted parent:\n%s", got)
	}
}

// TestInjectStackTablePrependsWhenAbsent pins the first-run case:
// a body with no sentinels gets the table inserted at the top with
// a blank-line separator so the user's prose still reads cleanly.
func TestInjectStackTablePrependsWhenAbsent(t *testing.T) {
	body := "Implements the login flow.\n\nFixes #99."
	table := stackTableStart + "\nstack content\n" + stackTableEnd
	got := injectStackTable(body, table)

	if !strings.HasPrefix(got, stackTableStart) {
		t.Fatalf("table must be at the top:\n%s", got)
	}
	if !strings.Contains(got, "Implements the login flow.") {
		t.Fatalf("original body must be preserved:\n%s", got)
	}
}

// TestInjectStackTableReplacesExistingBlock pins the idempotency
// contract: a body that already has a stac-man block sees the block
// REPLACED in-place, not duplicated. Without this the body would
// double in size on every `sm submit` re-run.
func TestInjectStackTableReplacesExistingBlock(t *testing.T) {
	body := "Top prose.\n\n" + stackTableStart + "\nold table\n" + stackTableEnd + "\n\nBottom prose."
	newTable := stackTableStart + "\nfresh table\n" + stackTableEnd
	got := injectStackTable(body, newTable)

	if strings.Contains(got, "old table") {
		t.Fatalf("old block must be replaced, found in:\n%s", got)
	}
	if !strings.Contains(got, "fresh table") {
		t.Fatalf("new block missing from:\n%s", got)
	}
	if !strings.Contains(got, "Top prose.") || !strings.Contains(got, "Bottom prose.") {
		t.Fatalf("manual prose around the block must survive:\n%s", got)
	}
	if n := strings.Count(got, stackTableStart); n != 1 {
		t.Fatalf("expected exactly one start sentinel after replace, got %d:\n%s", n, got)
	}
}

// TestInjectStackTableIdempotent pins the stronger contract:
// running injectStackTable twice in a row with the same table
// produces the same body. This is what makes `sm submit` safe to
// re-run any number of times — no body churn, no `gh pr edit`
// thrash, no review-noise notifications.
func TestInjectStackTableIdempotent(t *testing.T) {
	body := "Implements feature X."
	table := stackTableStart + "\nfresh table\n" + stackTableEnd
	once := injectStackTable(body, table)
	twice := injectStackTable(once, table)
	if once != twice {
		t.Fatalf("injectStackTable not idempotent:\nonce:\n%s\n\ntwice:\n%s", once, twice)
	}
}

// TestInjectStackTableEmptyTableNoOp pins the "no stack to render"
// path: when renderStackTable returns "" (single-branch stack), the
// existing body must be returned unchanged, sentinels and all.
func TestInjectStackTableEmptyTableNoOp(t *testing.T) {
	body := "Original body."
	if got := injectStackTable(body, ""); got != body {
		t.Fatalf("empty table should leave body alone, got %q", got)
	}
}

// TestInjectStackTableEmptyBodyNoSeparator pins the "fresh PR with
// empty body" path: there's nothing for the table to sit above of,
// so no blank-line separator should be appended after it. The
// resulting body is exactly the table.
func TestInjectStackTableEmptyBodyNoSeparator(t *testing.T) {
	table := stackTableStart + "\nfresh table\n" + stackTableEnd
	got := injectStackTable("", table)
	if got != table {
		t.Fatalf("empty body should produce just the table, got %q", got)
	}
}

// branchNameSlice is a small helper that mirrors branchNames in
// submit_test.go but takes the local stack.Branch type. Pulled out
// here so this file doesn't depend on the test ordering of submit_test.go.
func branchNameSlice(bs []stack.Branch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}
