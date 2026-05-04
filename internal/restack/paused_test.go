package restack

import (
	"context"
	"errors"
	"testing"
)

// TestPausedReturnsAbsentWhenNoStateFile pins the "clean repo" path:
// no restack.json on disk → (nil, false, nil), with no garbage left
// in the returned info.
func TestPausedReturnsAbsentWhenNoStateFile(t *testing.T) {
	g := newFakeGit(t)
	_, st := newGraph(t)
	e := New(g, st)

	info, ok, err := e.Paused(context.Background())
	if err != nil {
		t.Fatalf("Paused: %v", err)
	}
	if ok {
		t.Fatalf("ok = true, want false (no restack.json on disk)")
	}
	if info != nil {
		t.Fatalf("info = %+v, want nil when ok=false", info)
	}
}

// TestPausedReadsStateAndConflictPaths drives the full happy-path
// shape: a real restack pause leaves restack.json on disk, then
// Paused must surface the head of the queue as Branch, the full
// queue as Pending, and the live conflict paths from git.
func TestPausedReadsStateAndConflictPaths(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	g.conflictPaths = []string{"internal/foo.go", "internal/bar.go"}
	_, st := newGraph(t)
	e := New(g, st)

	if err := e.Restack(context.Background(), "feat-a"); err == nil {
		t.Fatalf("expected paused restack to error")
	}

	info, ok, err := e.Paused(context.Background())
	if err != nil {
		t.Fatalf("Paused: %v", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true after a paused restack")
	}
	if info.Origin != "restack" {
		t.Fatalf("Origin = %q, want %q", info.Origin, "restack")
	}
	if info.Branch != "feat-b" {
		t.Fatalf("Branch = %q, want feat-b (head of pending)", info.Branch)
	}
	wantPending := []string{"feat-b"}
	if !equalStringSlice(info.Pending, wantPending) {
		t.Fatalf("Pending = %v, want %v", info.Pending, wantPending)
	}
	if info.SavedBranch != "feat-a" {
		t.Fatalf("SavedBranch = %q, want feat-a", info.SavedBranch)
	}
	wantConflicts := []string{"internal/foo.go", "internal/bar.go"}
	if !equalStringSlice(info.ConflictPaths, wantConflicts) {
		t.Fatalf("ConflictPaths = %v, want %v", info.ConflictPaths, wantConflicts)
	}
}

// TestPausedConflictPathsErrorIsFatal locks down the fail-loud
// contract: a corrupt index (or any other ConflictPaths failure)
// must surface as a real error rather than masking the resolver
// behind a half-populated PausedInfo.
func TestPausedConflictPathsErrorIsFatal(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	g.conflictPathsErr = errors.New("index broken")
	_, st := newGraph(t)
	e := New(g, st)

	_ = e.Restack(context.Background(), "feat-a")

	if _, _, err := e.Paused(context.Background()); err == nil {
		t.Fatalf("Paused returned nil error, want propagation of ConflictPaths failure")
	}
}

// TestPausedConflictPathsEmptyIsFine covers the "user has staged
// every resolution but hasn't yet run continue" state: pending queue
// non-empty, conflicts empty. The resolver will use this combination
// to tell the user "ready — press c to continue".
func TestPausedConflictPathsEmptyIsFine(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	// conflictPaths nil → fakeGit returns []string{}.
	_, st := newGraph(t)
	e := New(g, st)

	_ = e.Restack(context.Background(), "feat-a")

	info, ok, err := e.Paused(context.Background())
	if err != nil {
		t.Fatalf("Paused: %v", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true")
	}
	if info.ConflictPaths == nil {
		t.Fatalf("ConflictPaths = nil, want empty slice")
	}
	if len(info.ConflictPaths) != 0 {
		t.Fatalf("ConflictPaths = %v, want empty", info.ConflictPaths)
	}
	if info.Branch != "feat-b" {
		t.Fatalf("Branch = %q, want feat-b", info.Branch)
	}
}

func equalStringSlice(a, b []string) bool {
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
