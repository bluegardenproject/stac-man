package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// snapshotRunner extends the bare scriptedRunner with two-arg
// matching for `rev-parse --git-dir` so paused-state lookups can
// hit a controllable directory in tests. The base case (one-arg
// `rev-parse <ref>` from CheckoutTree's needs-restack probes)
// still returns a stable SHA so the snapshot path stays exercised.
//
// conflictPaths is what `git diff --name-only --diff-filter=U`
// returns; the snapshot's Paused.ConflictPaths field reads through
// to this so tests can drive the resolver-screen contract.
type snapshotRunner struct {
	current       string
	gitDir        string
	conflictPaths string
}

func (s snapshotRunner) Run(_ context.Context, args ...string) (string, string, error) {
	if len(args) == 0 {
		return "", "", nil
	}
	switch {
	case args[0] == "rev-parse" && len(args) >= 2 && args[1] == "--is-inside-work-tree":
		return "true", "", nil
	case args[0] == "rev-parse" && len(args) >= 2 && args[1] == "--git-dir":
		return s.gitDir, "", nil
	case args[0] == "symbolic-ref":
		return s.current, "", nil
	case args[0] == "rev-parse":
		// Any other rev-parse (resolving a parent SHA for the
		// needs-restack probe) returns a stable, fake SHA so the
		// matches against BranchMeta.ParentSHA are deterministic.
		return "deadbeef", "", nil
	case args[0] == "diff" && len(args) >= 3 && args[1] == "--name-only" && args[2] == "--diff-filter=U":
		return s.conflictPaths, "", nil
	}
	return "", "", nil
}

func newSnapshotService(t *testing.T, current, gitDir string) (*Service, store.Store) {
	t.Helper()
	return newSnapshotServiceWithRunner(t, snapshotRunner{current: current, gitDir: gitDir})
}

func newSnapshotServiceWithRunner(t *testing.T, r snapshotRunner) (*Service, store.Store) {
	t.Helper()
	mem := memory.New()
	if err := mem.SetRepo(context.Background(), store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	return &Service{
		G:     git.NewWithRunner(r),
		Store: mem,
	}, mem
}

// TestSnapshotEmptyStackNoPaused locks down the happy-path zero
// state: trunk only, no children tracked, no paused restack. The
// cockpit's first-launch refresh has to land on this without
// erroring or returning a nil Log.
func TestSnapshotEmptyStackNoPaused(t *testing.T) {
	svc, _ := newSnapshotService(t, "main", t.TempDir())

	snap, err := svc.Snapshot(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Trunk != "main" {
		t.Fatalf("Trunk = %q, want main", snap.Trunk)
	}
	if snap.Current != "main" {
		t.Fatalf("Current = %q, want main", snap.Current)
	}
	if snap.Log == nil {
		t.Fatalf("Log is nil; the cockpit needs a non-nil graph even when empty")
	}
	if len(snap.Log.Branches) != 0 {
		t.Fatalf("Log.Branches = %d entries, want 0 on an empty stack", len(snap.Log.Branches))
	}
	if len(snap.CheckoutItems) != 1 || !snap.CheckoutItems[0].IsTrunk {
		t.Fatalf("CheckoutItems = %+v, want a single trunk row", snap.CheckoutItems)
	}
	if snap.Paused != nil {
		t.Fatalf("Paused = %+v, want nil when no restack.json exists", snap.Paused)
	}
}

// TestSnapshotMultiBranchTreeAtomic guarantees that Log and
// CheckoutItems describe the *same* graph in a single Snapshot.
// If a future refactor splits the underlying load into two reads
// against drifted state we'd see Log entries that don't appear in
// CheckoutItems (or vice versa) — this test catches that.
func TestSnapshotMultiBranchTreeAtomic(t *testing.T) {
	svc, st := newSnapshotService(t, "feat-b", t.TempDir())
	track(t, st, "feat-a", "main")
	track(t, st, "feat-b", "feat-a")
	track(t, st, "feat-c", "main")

	snap, err := svc.Snapshot(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	logBranches := map[string]bool{}
	for _, b := range snap.Log.Branches {
		logBranches[b.Branch] = true
	}
	checkoutBranches := map[string]bool{}
	for _, it := range snap.CheckoutItems {
		if it.IsTrunk {
			continue
		}
		checkoutBranches[it.Branch] = true
	}
	for name := range logBranches {
		if !checkoutBranches[name] {
			t.Fatalf("branch %q in Log but missing from CheckoutItems (snapshot is not atomic)", name)
		}
	}
	for name := range checkoutBranches {
		if !logBranches[name] {
			t.Fatalf("branch %q in CheckoutItems but missing from Log (snapshot is not atomic)", name)
		}
	}

	if snap.Current != "feat-b" {
		t.Fatalf("Current = %q, want feat-b", snap.Current)
	}
	// LogResult marks feat-b as current; CheckoutItems should agree.
	for _, b := range snap.Log.Branches {
		if b.Branch == "feat-b" && !b.IsCurrent {
			t.Fatalf("Log.Branches[feat-b].IsCurrent = false, want true")
		}
	}
	for _, it := range snap.CheckoutItems {
		if it.Branch == "feat-b" && !it.IsCurrent {
			t.Fatalf("CheckoutItems[feat-b].IsCurrent = false, want true")
		}
	}
}

// TestSnapshotSurfacesPausedState writes a synthetic restack.json
// into the test git dir and verifies Snapshot translates it into a
// PausedSnapshot, ConflictPaths included. The cockpit's conflict
// resolver routes off snap.Paused != nil and renders Pending +
// ConflictPaths verbatim, so both have to round-trip.
func TestSnapshotSurfacesPausedState(t *testing.T) {
	gitDir := t.TempDir()
	svc, st := newSnapshotServiceWithRunner(t, snapshotRunner{
		current:       "feat-a",
		gitDir:        gitDir,
		conflictPaths: "internal/foo.go\ninternal/bar.go\n",
	})
	track(t, st, "feat-a", "main")
	track(t, st, "feat-b", "feat-a")

	state := &restack.State{
		Origin:      "restack",
		SavedBranch: "feat-a",
		SavedTip:    "abc123",
		Pending: []restack.PendingBranch{
			{Name: "feat-a", OldParent: "main", NewParent: "main", OldSHA: "deadbeef"},
			{Name: "feat-b", OldParent: "feat-a", NewParent: "feat-a", OldSHA: "deadbeef"},
		},
	}
	if err := restack.Save(gitDir, state); err != nil {
		t.Fatalf("seeding restack.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(gitDir, "stac-man", "restack.json")); err != nil {
		t.Fatalf("seeded state missing: %v", err)
	}

	snap, err := svc.Snapshot(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if snap.Paused == nil {
		t.Fatalf("Paused = nil, want a snapshot of the seeded restack.json")
	}
	if snap.Paused.Origin != "restack" {
		t.Fatalf("Paused.Origin = %q, want %q", snap.Paused.Origin, "restack")
	}
	if snap.Paused.Branch != "feat-a" {
		t.Fatalf("Paused.Branch = %q, want %q (head of pending queue)", snap.Paused.Branch, "feat-a")
	}
	if snap.Paused.SavedBranch != "feat-a" {
		t.Fatalf("Paused.SavedBranch = %q, want %q", snap.Paused.SavedBranch, "feat-a")
	}
	wantPending := []string{"feat-a", "feat-b"}
	if len(snap.Paused.Pending) != len(wantPending) {
		t.Fatalf("Paused.Pending = %v, want %v", snap.Paused.Pending, wantPending)
	}
	for i, want := range wantPending {
		if snap.Paused.Pending[i] != want {
			t.Fatalf("Paused.Pending[%d] = %q, want %q", i, snap.Paused.Pending[i], want)
		}
	}
	wantConflicts := []string{"internal/foo.go", "internal/bar.go"}
	if len(snap.Paused.ConflictPaths) != len(wantConflicts) {
		t.Fatalf("Paused.ConflictPaths = %v, want %v", snap.Paused.ConflictPaths, wantConflicts)
	}
	for i, want := range wantConflicts {
		if snap.Paused.ConflictPaths[i] != want {
			t.Fatalf("Paused.ConflictPaths[%d] = %q, want %q", i, snap.Paused.ConflictPaths[i], want)
		}
	}
}

// TestPausedStateAbsentReturnsNil pins the resolver-screen contract
// when the user is not paused: the cockpit periodically polls
// PausedState while the resolver is open, and a nil return tells it
// the rebase finished elsewhere (e.g. via `sm continue` from another
// terminal) so the screen should close itself.
func TestPausedStateAbsentReturnsNil(t *testing.T) {
	svc, _ := newSnapshotService(t, "main", t.TempDir())
	got, err := svc.PausedState(context.Background())
	if err != nil {
		t.Fatalf("PausedState: %v", err)
	}
	if got != nil {
		t.Fatalf("PausedState = %+v, want nil when no restack.json exists", got)
	}
}

// TestPausedStateMatchesSnapshot guarantees the standalone
// PausedState call returns the same shape as Snapshot.Paused when
// both run against the same git dir / store. Without this it would
// be possible for a future refactor to drift the two readers apart
// and have the resolver screen disagree with the dashboard.
func TestPausedStateMatchesSnapshot(t *testing.T) {
	gitDir := t.TempDir()
	svc, st := newSnapshotServiceWithRunner(t, snapshotRunner{
		current:       "feat-a",
		gitDir:        gitDir,
		conflictPaths: "x.go\n",
	})
	track(t, st, "feat-a", "main")

	state := &restack.State{
		Origin:      "restack",
		SavedBranch: "feat-a",
		Pending: []restack.PendingBranch{
			{Name: "feat-a", OldParent: "main", NewParent: "main"},
		},
	}
	if err := restack.Save(gitDir, state); err != nil {
		t.Fatalf("seeding restack.json: %v", err)
	}

	snap, err := svc.Snapshot(context.Background(), LogOptions{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	standalone, err := svc.PausedState(context.Background())
	if err != nil {
		t.Fatalf("PausedState: %v", err)
	}
	if snap.Paused == nil || standalone == nil {
		t.Fatalf("expected both readers to surface the paused state; snap=%+v standalone=%+v", snap.Paused, standalone)
	}
	if snap.Paused.Branch != standalone.Branch ||
		snap.Paused.Origin != standalone.Origin ||
		snap.Paused.SavedBranch != standalone.SavedBranch {
		t.Fatalf("Snapshot.Paused (%+v) and PausedState (%+v) disagree on scalar fields", snap.Paused, standalone)
	}
}
