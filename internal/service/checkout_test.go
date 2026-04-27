package service

import (
	"context"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// scriptedRunner returns canned responses for git invocations,
// matched by the leading argument so tests can stay short. Calls
// that don't match a key fall back to an empty success — that's
// enough for the tree-shape assertions in this file.
type scriptedRunner struct {
	byCmd map[string]string
}

func (s scriptedRunner) Run(_ context.Context, args ...string) (string, string, error) {
	if len(args) == 0 {
		return "", "", nil
	}
	if out, ok := s.byCmd[args[0]]; ok {
		return out, "", nil
	}
	return "", "", nil
}

func newCheckoutService(t *testing.T, current string) (*Service, store.Store) {
	t.Helper()
	mem := memory.New()
	if err := mem.SetRepo(context.Background(), store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	runner := scriptedRunner{byCmd: map[string]string{
		"rev-parse":    "true",
		"symbolic-ref": current,
	}}
	return &Service{G: git.NewWithRunner(runner), Store: mem}, mem
}

func track(t *testing.T, s store.Store, branch, parent string) {
	t.Helper()
	if err := s.SetBranch(context.Background(), branch, store.BranchMeta{Parent: parent}); err != nil {
		t.Fatalf("SetBranch %s: %v", branch, err)
	}
}

func TestCheckoutTreeTrunkOnly(t *testing.T) {
	svc, _ := newCheckoutService(t, "main")
	current, items, err := svc.CheckoutTree(context.Background())
	if err != nil {
		t.Fatalf("CheckoutTree: %v", err)
	}
	if current != "main" {
		t.Fatalf("current = %q, want main", current)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	got := items[0]
	if !got.IsTrunk || got.Branch != "main" || !got.IsCurrent || got.Depth != 0 {
		t.Fatalf("unexpected trunk row: %+v", got)
	}
}

func TestCheckoutTreeLinearChain(t *testing.T) {
	svc, st := newCheckoutService(t, "feat-c")
	track(t, st, "feat-a", "main")
	track(t, st, "feat-b", "feat-a")
	track(t, st, "feat-c", "feat-b")

	_, items, err := svc.CheckoutTree(context.Background())
	if err != nil {
		t.Fatalf("CheckoutTree: %v", err)
	}
	wantNames := []string{"main", "feat-a", "feat-b", "feat-c"}
	wantDepths := []int{0, 1, 2, 3}
	if len(items) != len(wantNames) {
		t.Fatalf("got %d items, want %d", len(items), len(wantNames))
	}
	for i, want := range wantNames {
		if items[i].Branch != want {
			t.Fatalf("items[%d].Branch = %q, want %q", i, items[i].Branch, want)
		}
		if items[i].Depth != wantDepths[i] {
			t.Fatalf("items[%d].Depth = %d, want %d", i, items[i].Depth, wantDepths[i])
		}
	}

	// The leaf is the only current branch and the only IsLastChild
	// at its depth (single chain).
	leaf := items[len(items)-1]
	if !leaf.IsCurrent {
		t.Fatalf("expected feat-c to be current")
	}
	if !leaf.IsLastChild {
		t.Fatalf("leaf should be marked IsLastChild")
	}
	// In a linear chain every non-trunk row's ancestors are all
	// last-children — that's what makes the connectors collapse to
	// straight padding instead of pipes.
	for _, last := range leaf.AncestorIsLast {
		if !last {
			t.Fatalf("expected all ancestors to be last-children, got %v", leaf.AncestorIsLast)
		}
	}
}

func TestCheckoutTreeForkAtRoot(t *testing.T) {
	svc, st := newCheckoutService(t, "feat-b")
	track(t, st, "feat-a", "main")
	track(t, st, "feat-b", "main")

	_, items, err := svc.CheckoutTree(context.Background())
	if err != nil {
		t.Fatalf("CheckoutTree: %v", err)
	}
	wantNames := []string{"main", "feat-a", "feat-b"}
	for i, want := range wantNames {
		if items[i].Branch != want {
			t.Fatalf("items[%d].Branch = %q, want %q", i, items[i].Branch, want)
		}
	}
	// feat-a is the first of two siblings → not the last child.
	if items[1].IsLastChild {
		t.Fatalf("feat-a should not be the last child of its parent")
	}
	// feat-b is the alphabetically-last sibling → IsLastChild.
	if !items[2].IsLastChild {
		t.Fatalf("feat-b should be the last child of its parent")
	}
}

func TestCheckoutTreeNestedFork(t *testing.T) {
	svc, st := newCheckoutService(t, "main")
	// main
	//  └─ feat-a
	//      ├─ feat-b
	//      │   └─ feat-d
	//      └─ feat-c
	track(t, st, "feat-a", "main")
	track(t, st, "feat-b", "feat-a")
	track(t, st, "feat-c", "feat-a")
	track(t, st, "feat-d", "feat-b")

	_, items, err := svc.CheckoutTree(context.Background())
	if err != nil {
		t.Fatalf("CheckoutTree: %v", err)
	}
	wantNames := []string{"main", "feat-a", "feat-b", "feat-d", "feat-c"}
	for i, want := range wantNames {
		if items[i].Branch != want {
			t.Fatalf("items[%d].Branch = %q, want %q (%v)", i, items[i].Branch, want, names(items))
		}
	}
	// feat-d sits under feat-b (not last among feat-a's children),
	// so its ancestors-is-last vector should be [true(feat-a is last
	// child of trunk), false(feat-b is *not* the last child of feat-a)].
	d := items[3]
	if got, want := d.AncestorIsLast, []bool{true, false}; !equalBools(got, want) {
		t.Fatalf("feat-d AncestorIsLast = %v, want %v", got, want)
	}
	if !d.IsLastChild {
		t.Fatalf("feat-d should be the last (only) child of feat-b")
	}
	// feat-c is feat-a's last child → IsLastChild=true,
	// AncestorIsLast=[true] (just the trunk's last-child branch above it).
	c := items[4]
	if !c.IsLastChild {
		t.Fatalf("feat-c should be IsLastChild")
	}
	if got, want := c.AncestorIsLast, []bool{true}; !equalBools(got, want) {
		t.Fatalf("feat-c AncestorIsLast = %v, want %v", got, want)
	}
}

func TestCheckoutTreeNoCurrentMatchOnDetachedHead(t *testing.T) {
	// symbolic-ref returns empty when HEAD is detached; mock that.
	svc, st := newCheckoutService(t, "")
	track(t, st, "feat-a", "main")

	current, items, err := svc.CheckoutTree(context.Background())
	if err != nil {
		t.Fatalf("CheckoutTree: %v", err)
	}
	if current != "" {
		t.Fatalf("expected empty current on detached HEAD, got %q", current)
	}
	for _, it := range items {
		if it.IsCurrent {
			t.Fatalf("no row should be marked current on detached HEAD: %+v", it)
		}
	}
}

func names(items []CheckoutItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Branch
	}
	return out
}

func equalBools(a, b []bool) bool {
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
