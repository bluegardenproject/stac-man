package restack

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// fakeGit is a minimal Git implementation for the engine tests. It
// records the rebase call sequence and lets tests inject a conflict
// at a given step.
type fakeGit struct {
	gitDir string
	tips   map[string]string // ref -> SHA

	// conflictOn, when non-empty, makes Rebase return an error the
	// first time `branch` matches that name.
	conflictOn string

	rebases    []rebaseCall
	checkouts  []string
	continued  int
	aborted    int
	inProgress bool
}

type rebaseCall struct {
	onto, upstream, branch string
}

func (f *fakeGit) GitDir(_ context.Context) (string, error)        { return f.gitDir, nil }
func (f *fakeGit) CurrentBranch(_ context.Context) (string, error) { return "feat-a", nil }
func (f *fakeGit) Checkout(_ context.Context, branch string) error {
	f.checkouts = append(f.checkouts, branch)
	return nil
}
func (f *fakeGit) RebaseInProgress(_ context.Context) (bool, error) { return f.inProgress, nil }
func (f *fakeGit) RebaseAbort(_ context.Context) error              { f.aborted++; f.inProgress = false; return nil }
func (f *fakeGit) RebaseContinue(_ context.Context) error {
	f.continued++
	f.inProgress = false
	return nil
}

func (f *fakeGit) RevParse(_ context.Context, ref string) (string, error) {
	if sha, ok := f.tips[ref]; ok {
		return sha, nil
	}
	return "", errors.New("unknown ref " + ref)
}

func (f *fakeGit) Rebase(_ context.Context, onto, upstream, branch string) error {
	f.rebases = append(f.rebases, rebaseCall{onto, upstream, branch})
	if branch == f.conflictOn {
		f.inProgress = true
		return errors.New("conflict")
	}
	// Successful rebase advances the rebased branch's SHA so a later
	// recordSuccess sees a fresh tip.
	f.tips[branch] = "post-rebase-" + branch
	return nil
}

func newFakeGit(t *testing.T) *fakeGit {
	t.Helper()
	return &fakeGit{
		gitDir: filepath.Join(t.TempDir(), ".git"),
		tips: map[string]string{
			"main":   "main-tip",
			"feat-a": "feat-a-old",
			"feat-b": "feat-b-old",
		},
	}
}

func newGraph(t *testing.T) (*memory.Store, store.Store) {
	t.Helper()
	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", ParentSHA: "main-old"}); err != nil {
		t.Fatalf("SetBranch feat-a: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-b", store.BranchMeta{Parent: "feat-a", ParentSHA: "feat-a-old"}); err != nil {
		t.Fatalf("SetBranch feat-b: %v", err)
	}
	return mem, mem
}

func TestRestackHappyPath(t *testing.T) {
	g := newFakeGit(t)
	mem, st := newGraph(t)
	e := New(g, st)

	if err := e.Restack(context.Background(), "feat-a"); err != nil {
		t.Fatalf("Restack: %v", err)
	}

	// Both branches should have been rebased onto their (fresh) parents.
	if len(g.rebases) != 2 {
		t.Fatalf("expected 2 rebase calls, got %d: %+v", len(g.rebases), g.rebases)
	}
	if g.rebases[0].branch != "feat-a" || g.rebases[1].branch != "feat-b" {
		t.Fatalf("topo order wrong: %+v", g.rebases)
	}

	// State file should be cleared on success.
	if _, ok, _ := Load(g.gitDir); ok {
		t.Fatalf("expected restack state to be cleared")
	}

	// ParentSHA should now reflect the new tips.
	meta, _, _ := mem.GetBranch(context.Background(), "feat-b")
	if meta.ParentSHA != "post-rebase-feat-a" {
		t.Fatalf("feat-b.ParentSHA = %q, want post-rebase-feat-a", meta.ParentSHA)
	}
}

func TestRestackConflictPersistsState(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	mem, st := newGraph(t)
	e := New(g, st)

	err := e.Restack(context.Background(), "feat-a")
	var paused *PausedError
	if !errors.As(err, &paused) {
		t.Fatalf("expected PausedError, got %v", err)
	}
	if paused.Branch != "feat-b" {
		t.Fatalf("paused branch %q, want feat-b", paused.Branch)
	}

	// State on disk should mark feat-b as the head of pending.
	stOnDisk, ok, err := Load(g.gitDir)
	if err != nil || !ok {
		t.Fatalf("expected restack state on disk: ok=%v err=%v", ok, err)
	}
	if len(stOnDisk.Pending) == 0 || stOnDisk.Pending[0].Name != "feat-b" {
		t.Fatalf("expected feat-b at head of pending, got %+v", stOnDisk.Pending)
	}

	// feat-a's parent SHA should already be updated even though feat-b
	// failed — its rebase succeeded.
	meta, _, _ := mem.GetBranch(context.Background(), "feat-a")
	if meta.ParentSHA != "main-tip" {
		t.Fatalf("feat-a.ParentSHA = %q, want main-tip", meta.ParentSHA)
	}
}

func TestContinueResumesRemaining(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	mem, st := newGraph(t)
	e := New(g, st)

	_ = e.Restack(context.Background(), "feat-a") // pauses on feat-b

	// Simulate the user resolving the conflict and rerunning sm continue.
	g.conflictOn = ""               // no more conflicts
	g.tips["feat-b"] = "feat-b-new" // git rebase --continue would advance this
	if err := e.Continue(context.Background()); err != nil {
		t.Fatalf("Continue: %v", err)
	}
	if g.continued != 1 {
		t.Fatalf("expected RebaseContinue to be called once, got %d", g.continued)
	}
	if _, ok, _ := Load(g.gitDir); ok {
		t.Fatalf("expected state cleared after successful continue")
	}
	meta, _, _ := mem.GetBranch(context.Background(), "feat-b")
	if meta.ParentSHA != "post-rebase-feat-a" {
		t.Fatalf("feat-b.ParentSHA after continue = %q, want post-rebase-feat-a", meta.ParentSHA)
	}
}

func TestAbortRestoresAndClears(t *testing.T) {
	g := newFakeGit(t)
	g.conflictOn = "feat-b"
	mem, st := newGraph(t)
	e := New(g, st)
	_ = mem // keep mem from being collected; not needed beyond setup

	_ = e.Restack(context.Background(), "feat-a") // pauses on feat-b
	if err := e.Abort(context.Background()); err != nil {
		t.Fatalf("Abort: %v", err)
	}
	if g.aborted != 1 {
		t.Fatalf("expected RebaseAbort to be called, got %d", g.aborted)
	}
	if _, ok, _ := Load(g.gitDir); ok {
		t.Fatalf("expected state cleared after abort")
	}
	if len(g.checkouts) == 0 || g.checkouts[len(g.checkouts)-1] != "feat-a" {
		t.Fatalf("expected checkout back to saved branch, got %v", g.checkouts)
	}
}

func TestContinueWithoutStateErrors(t *testing.T) {
	g := newFakeGit(t)
	mem, st := newGraph(t)
	_ = mem
	e := New(g, st)
	if err := e.Continue(context.Background()); err == nil {
		t.Fatalf("expected error when no paused state exists")
	}
}

func TestRestackSkipsWhenNothingMoved(t *testing.T) {
	g := newFakeGit(t)
	mem, st := newGraph(t)
	// Both branches' recorded ParentSHAs match the live tips.
	_ = mem.SetBranch(context.Background(), "feat-a", store.BranchMeta{
		Parent:    "main",
		ParentSHA: "main-tip",
	})
	_ = mem.SetBranch(context.Background(), "feat-b", store.BranchMeta{
		Parent:    "feat-a",
		ParentSHA: "feat-a-old",
	})
	e := New(g, st)
	if err := e.Restack(context.Background(), "feat-a"); err != nil {
		t.Fatalf("Restack: %v", err)
	}
	if len(g.rebases) != 0 {
		t.Fatalf("expected no rebases when graph is up-to-date, got %+v", g.rebases)
	}
}

func TestRestackPropagatesThroughChain(t *testing.T) {
	g := newFakeGit(t)
	mem, st := newGraph(t)
	// Only feat-a's ParentSHA is stale; feat-b's ParentSHA is current.
	// Restacking feat-a must still rebase feat-b because feat-a will
	// move underneath it.
	_ = mem.SetBranch(context.Background(), "feat-a", store.BranchMeta{
		Parent:    "main",
		ParentSHA: "main-old",
	})
	_ = mem.SetBranch(context.Background(), "feat-b", store.BranchMeta{
		Parent:    "feat-a",
		ParentSHA: "feat-a-old",
	})
	e := New(g, st)
	if err := e.Restack(context.Background(), "feat-a"); err != nil {
		t.Fatalf("Restack: %v", err)
	}
	if len(g.rebases) != 2 {
		t.Fatalf("expected 2 rebases (chain propagation), got %+v", g.rebases)
	}
}
