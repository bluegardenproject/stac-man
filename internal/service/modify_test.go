package service

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// exitOne is a real *exec.ExitError with code 1. The git client's
// ExitErrorOf check is implemented with errors.As(*exec.ExitError),
// so a plain errors.New("...") wouldn't be recognized as a 1-exit.
var exitOne error

func init() {
	cmd := exec.Command("false")
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitOne = ee
		}
	}
	if exitOne == nil {
		// Fall back to a plain error so package init doesn't fail on
		// platforms without /usr/bin/false. Tests that need a 1-exit
		// will then see a different code path; protect them with a
		// guard below in TestDoctorReportsDriftedParent.
		exitOne = errors.New("exit status 1")
	}
}

// fakeRunner is the local equivalent of the unexported runner used in
// internal/git tests. We only key by full argv so a typo in production
// shows up as an unmocked call (returned from fallback).
type fakeRunner struct {
	calls        [][]string
	responses    map[string]string
	errResponses map[string]error
	fallback     string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	key := strings.Join(args, " ")
	if e, ok := f.errResponses[key]; ok {
		return "", "", e
	}
	if r, ok := f.responses[key]; ok {
		return r, "", nil
	}
	return f.fallback, "", nil
}

func (f *fakeRunner) called(prefix ...string) bool {
	for _, c := range f.calls {
		if len(c) < len(prefix) {
			continue
		}
		match := true
		for i, p := range prefix {
			if c[i] != p {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// newFakeService builds a Service whose git client is driven by a
// fakeRunner and whose store is in-memory, with the given current
// branch and trunk preconfigured.
func newFakeService(t *testing.T, current, trunk string, ahead int) (*Service, *fakeRunner) {
	t.Helper()
	// Route history writes into a per-test temp dir so the suite can't
	// litter the working copy with `stac-man/history.json`.
	gitDir := t.TempDir()
	r := &fakeRunner{
		responses: map[string]string{
			"rev-parse --is-inside-work-tree":            "true",
			"rev-parse --git-dir":                        gitDir,
			"symbolic-ref --short HEAD":                  current,
			"rev-list --count " + trunk + ".." + current: itoa(ahead),
		},
	}
	mem := memory.New()
	if err := mem.SetRepo(context.Background(), store.RepoMeta{Trunk: trunk, Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	s := &Service{G: git.NewWithRunner(r), Store: mem}
	return s, r
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestModifyAmendRefusedOnEmptyUntrackedBranch covers the case where
// the user runs `sm modify` (default amend) on a branch that has no
// commits of its own and isn't tracked. Without the guard, Modify
// would amend a commit that belongs to trunk.
func TestModifyAmendRefusedOnEmptyUntrackedBranch(t *testing.T) {
	s, r := newFakeService(t, "feat-empty", "main", 0)

	err := s.Modify(context.Background(), ModifyOptions{Amend: true})
	if err == nil {
		t.Fatalf("expected error when amending an empty branch")
	}
	if !strings.Contains(err.Error(), "no commits of its own") {
		t.Fatalf("error should mention empty branch, got: %v", err)
	}
	if r.called("commit", "--amend") {
		t.Fatalf("git commit --amend must not run when guard fires")
	}
}

// TestModifyAmendRefusedOnEmptyTrackedBranch is the bug we hit during
// manual testing: the branch is tracked under a parent, but the tip
// equals the parent SHA, so amending would rewrite the parent's commit.
func TestModifyAmendRefusedOnEmptyTrackedBranch(t *testing.T) {
	s, r := newFakeService(t, "feat-empty", "main", 0)
	// Track the branch under feat-base (which itself is on main).
	mem := s.Store.(*memory.Store)
	if err := mem.SetBranch(context.Background(), "feat-base", store.BranchMeta{Parent: "main"}); err != nil {
		t.Fatalf("SetBranch base: %v", err)
	}
	if err := mem.SetBranch(context.Background(), "feat-empty", store.BranchMeta{Parent: "feat-base"}); err != nil {
		t.Fatalf("SetBranch empty: %v", err)
	}
	// Re-key the rev-list response so the count is checked against the
	// recorded parent (feat-base) instead of trunk.
	r.responses["rev-list --count feat-base..feat-empty"] = "0"

	err := s.Modify(context.Background(), ModifyOptions{Amend: true})
	if err == nil {
		t.Fatalf("expected error when amending an empty tracked branch")
	}
	if !strings.Contains(err.Error(), "feat-base") {
		t.Fatalf("error should reference the parent branch, got: %v", err)
	}
	if r.called("commit", "--amend") {
		t.Fatalf("git commit --amend must not run when guard fires")
	}
}

// TestModifyCommitWorksOnEmptyBranch makes sure the workaround we tell
// users about in the error message — `sm modify -c -m ...` — actually
// goes through to a plain `git commit`.
func TestModifyCommitWorksOnEmptyBranch(t *testing.T) {
	s, r := newFakeService(t, "feat-empty", "main", 0)

	err := s.Modify(context.Background(), ModifyOptions{Commit: true, Message: "first"})
	if err != nil {
		t.Fatalf("Modify with -c on empty branch: %v", err)
	}
	if !r.called("commit", "-m", "first") {
		t.Fatalf("expected `git commit -m first`, calls: %v", r.calls)
	}
	if r.called("commit", "--amend") {
		t.Fatalf("must not amend when -c is set")
	}
}

// TestModifyAmendAllowedWhenBranchHasOwnCommits is the happy path: the
// guard must not fire when the branch already has at least one commit
// past its parent.
func TestModifyAmendAllowedWhenBranchHasOwnCommits(t *testing.T) {
	s, r := newFakeService(t, "feat-x", "main", 1)

	err := s.Modify(context.Background(), ModifyOptions{Amend: true})
	if err != nil {
		t.Fatalf("Modify amend: %v", err)
	}
	if !r.called("commit", "--amend") {
		t.Fatalf("expected `git commit --amend`, calls: %v", r.calls)
	}
}

// TestModifyStageAllUsesAddUpdate pins the B3 fix: the default `-a`
// path stages tracked-but-modified files only (`git add -u`), so a
// stray untracked file in the working tree is not silently absorbed
// into the commit.
func TestModifyStageAllUsesAddUpdate(t *testing.T) {
	s, r := newFakeService(t, "feat-x", "main", 1)

	err := s.Modify(context.Background(), ModifyOptions{Amend: true, StageAll: true})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if !r.called("add", "-u") {
		t.Fatalf("expected `git add -u`, calls: %v", r.calls)
	}
	if r.called("add", "-A") {
		t.Fatalf("must not run `git add -A` without --include-untracked, calls: %v", r.calls)
	}
}

// TestModifyStageAllWithUntrackedUsesAddAll verifies the opt-in path:
// passing IncludeUntracked alongside StageAll restores the previous
// "stage everything" behavior for users who genuinely want it.
func TestModifyStageAllWithUntrackedUsesAddAll(t *testing.T) {
	s, r := newFakeService(t, "feat-x", "main", 1)

	err := s.Modify(context.Background(), ModifyOptions{Amend: true, StageAll: true, IncludeUntracked: true})
	if err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if !r.called("add", "-A") {
		t.Fatalf("expected `git add -A` with --include-untracked, calls: %v", r.calls)
	}
	if r.called("add", "-u") {
		t.Fatalf("must not run both add invocations, calls: %v", r.calls)
	}
}

func TestModifyRestoresOriginalBranchAfterRestackingDescendant(t *testing.T) {
	s, r := newFakeService(t, "feat-a", "main", 1)
	mem := s.Store.(*memory.Store)
	if err := mem.SetBranch(context.Background(), "feat-a", store.BranchMeta{Parent: "main", ParentSHA: "main-old"}); err != nil {
		t.Fatalf("SetBranch feat-a: %v", err)
	}
	if err := mem.SetBranch(context.Background(), "feat-b", store.BranchMeta{Parent: "feat-a", ParentSHA: "feat-a-old"}); err != nil {
		t.Fatalf("SetBranch feat-b: %v", err)
	}
	r.responses["rev-parse --verify main^{commit}"] = "main-new"
	r.responses["rev-parse --verify feat-a^{commit}"] = "feat-a-new"

	if err := s.Modify(context.Background(), ModifyOptions{Amend: true, StageAll: true}); err != nil {
		t.Fatalf("Modify: %v", err)
	}
	if !r.called("rebase", "--onto", "feat-a-new", "feat-a-old", "feat-b") {
		t.Fatalf("expected descendant rebase, calls: %v", r.calls)
	}
	last := r.calls[len(r.calls)-1]
	if len(last) != 2 || last[0] != "checkout" || last[1] != "feat-a" {
		t.Fatalf("expected final checkout back to feat-a, calls: %v", r.calls)
	}
}

// stagerSpy is a minimal gitStager that records which staging variant
// was invoked. Lets us unit-test stageWorkingTree without spinning up
// a full fake runner.
type stagerSpy struct {
	addAllCalled    bool
	addUpdateCalled bool
}

func (s *stagerSpy) AddAll(_ context.Context) error    { s.addAllCalled = true; return nil }
func (s *stagerSpy) AddUpdate(_ context.Context) error { s.addUpdateCalled = true; return nil }

func TestStageWorkingTreeTrackedOnly(t *testing.T) {
	spy := &stagerSpy{}
	if err := stageWorkingTree(context.Background(), spy, false); err != nil {
		t.Fatalf("stageWorkingTree: %v", err)
	}
	if !spy.addUpdateCalled {
		t.Fatalf("expected AddUpdate to be called")
	}
	if spy.addAllCalled {
		t.Fatalf("AddAll must not be called when includeUntracked=false")
	}
}

func TestStageWorkingTreeIncludeUntracked(t *testing.T) {
	spy := &stagerSpy{}
	if err := stageWorkingTree(context.Background(), spy, true); err != nil {
		t.Fatalf("stageWorkingTree: %v", err)
	}
	if !spy.addAllCalled {
		t.Fatalf("expected AddAll to be called when includeUntracked=true")
	}
	if spy.addUpdateCalled {
		t.Fatalf("AddUpdate must not be called when includeUntracked=true")
	}
}
