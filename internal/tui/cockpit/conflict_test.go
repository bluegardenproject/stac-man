package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/service"
)

// snapshotPaused returns a snapshot whose Paused field describes
// a multi-path conflict on feat-a with feat-b queued behind it.
// Used as the canonical fixture for conflict-screen tests so each
// test starts with two paths (enough to exercise cursor movement
// and edit-target selection) and a non-empty pending queue.
func snapshotPaused() *service.DashboardSnapshot {
	return &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "feat-a",
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true},
			{Branch: "feat-a", Depth: 1, IsCurrent: true},
			{Branch: "feat-b", Depth: 1, IsLastChild: true},
		},
		Paused: &service.PausedSnapshot{
			Branch:        "feat-a",
			Origin:        "restack",
			Pending:       []string{"feat-b"},
			ConflictPaths: []string{"src/foo.go", "src/bar.go"},
		},
	}
}

// TestRouteFirstLoadIntoPausedJumpsToResolver pins the boot UX:
// launching `sm` while a rebase is paused must drop the user
// straight into the conflict resolver. Without this they'd have to
// notice the dashboard banner and hunt for the entry key.
func TestRouteFirstLoadIntoPausedJumpsToResolver(t *testing.T) {
	m := New(context.Background(), nil)
	next, _ := m.Update(snapshotMsg{snap: snapshotPaused()})
	got := next.(Model)
	if got.screen != screenConflict {
		t.Fatalf("screen = %v, want screenConflict on first paused snapshot", got.screen)
	}
	if got.conflictCursor != 0 {
		t.Fatalf("conflictCursor = %d, want 0 on entry", got.conflictCursor)
	}
}

// TestRouteSubsequentSnapshotKeepsUserChoice verifies that once the
// user has navigated to the dashboard (e.g. via Back from the
// resolver), subsequent snapshots that still show paused must NOT
// yank them back into the resolver. Auto-routing only happens on
// first load.
func TestRouteSubsequentSnapshotKeepsUserChoice(t *testing.T) {
	m := New(context.Background(), nil)
	// First snapshot routes into the resolver.
	next, _ := m.Update(snapshotMsg{snap: snapshotPaused()})
	m = next.(Model)
	// User pressed esc to browse the dashboard.
	m.screen = screenDashboard

	// A refresh hits while still paused.
	next2, _ := m.Update(snapshotMsg{snap: snapshotPaused()})
	got := next2.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v after user-driven Back + refresh, want screenDashboard (auto-routing must not override the user's choice)", got.screen)
	}
}

// TestRouteResolvedSnapshotBouncesBack covers the success-path
// transition: the resolver's continue/abort cleared the paused
// state, the next snapshot has Paused == nil, the cockpit must
// drop the user back on the dashboard automatically.
func TestRouteResolvedSnapshotBouncesBack(t *testing.T) {
	m := New(context.Background(), nil)
	// First snapshot lands in the resolver.
	next, _ := m.Update(snapshotMsg{snap: snapshotPaused()})
	m = next.(Model)
	if m.screen != screenConflict {
		t.Fatalf("setup: expected to land in conflict resolver, got screen %v", m.screen)
	}

	// Resolution complete — paused field cleared.
	resolved := snapshotPaused()
	resolved.Paused = nil
	next2, _ := m.Update(snapshotMsg{snap: resolved})
	got := next2.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v after Paused cleared, want screenDashboard", got.screen)
	}
	if got.conflictCursor != 0 {
		t.Fatalf("conflictCursor = %d after bounce-back, want reset to 0", got.conflictCursor)
	}
}

// TestActionResultPausedRoutesToResolver pins the in-session entry:
// any local action returning a *PausedError must move the user from
// the dashboard into the resolver immediately, before the snapshot
// reload even arrives. The reset of conflictCursor on entry is
// part of the contract so a stale cursor doesn't leak in.
func TestActionResultPausedRoutesToResolver(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.screen = screenDashboard
	m.conflictCursor = 99 // simulate stale state

	paused := &service.PausedError{Branch: "feat-a"}
	next, cmd := m.Update(actionResult{Verb: "restack", Branch: "feat-a", Err: paused, Paused: paused})
	got := next.(Model)
	if got.screen != screenConflict {
		t.Fatalf("screen = %v after paused actionResult, want screenConflict", got.screen)
	}
	if got.conflictCursor != 0 {
		t.Fatalf("conflictCursor = %d after entry, want 0 (reset)", got.conflictCursor)
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload cmd after paused, got nil")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg", cmd())
	}
}

// TestConflictCursorMovesAndClamps exercises the full nav-key
// matrix on the resolver: Down/End move forward, Up/Home move
// backward, both bound by the path list. Bounds violations are
// silent no-ops, not panics or wrap-arounds.
func TestConflictCursorMovesAndClamps(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	cases := []struct {
		name  string
		setup int
		key   string
		want  int
	}{
		{"down moves forward", 0, "down", 1},
		{"down at last clamps", 1, "down", 1},
		{"up moves backward", 1, "up", 0},
		{"up at top clamps", 0, "up", 0},
		{"end jumps to last", 0, "end", 1},
		{"home jumps to first", 1, "home", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m.conflictCursor = tc.setup
			next, _ := m.Update(syntheticKey(tc.key))
			got := next.(Model)
			if got.conflictCursor != tc.want {
				t.Fatalf("conflictCursor = %d, want %d", got.conflictCursor, tc.want)
			}
		})
	}
}

// TestConflictBackReturnsToDashboard pins the manual escape hatch
// from the resolver. Used when the user wants to inspect the
// dashboard while a rebase is paused (e.g. to confirm which
// children will be affected by an abort).
func TestConflictBackReturnsToDashboard(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	next, _ := m.Update(syntheticKey("esc"))
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v after esc, want screenDashboard", got.screen)
	}
}

// TestConflictContinueDispatches verifies the `c` binding fires a
// RestackContinue with the right verb label. Same dispatch-shape
// assertion as the local-action tests — service-layer outcomes
// belong to service tests.
func TestConflictContinueDispatches(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	_, cmd := m.Update(syntheticKey("c"))
	if cmd == nil {
		t.Fatalf("c key returned nil cmd, want a continue dispatch")
	}
	res, ok := cmd().(actionResult)
	if !ok {
		t.Fatalf("cmd() = %T, want actionResult", cmd())
	}
	if res.Verb != m.keys.ConflictContinue.Help {
		t.Fatalf("Verb = %q, want %q", res.Verb, m.keys.ConflictContinue.Help)
	}
}

// TestConflictAbortDispatches mirrors continue for the abort path
// so removing one binding without the other becomes a test failure
// rather than silent UX drift.
func TestConflictAbortDispatches(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	_, cmd := m.Update(syntheticKey("A"))
	if cmd == nil {
		t.Fatalf("A key returned nil cmd, want an abort dispatch")
	}
	res, ok := cmd().(actionResult)
	if !ok {
		t.Fatalf("cmd() = %T, want actionResult", cmd())
	}
	if res.Verb != m.keys.ConflictAbort.Help {
		t.Fatalf("Verb = %q, want %q", res.Verb, m.keys.ConflictAbort.Help)
	}
}

// TestConflictEditNoopWhenNoPaths covers the empty-conflict-list
// case: pressing `e` must not spawn a blank editor session. We can
// reach this state when the user has staged all conflicting files
// (git stops reporting them as unmerged) but hasn't yet pressed
// continue.
func TestConflictEditNoopWhenNoPaths(t *testing.T) {
	m := New(context.Background(), nil)
	snap := snapshotPaused()
	snap.Paused.ConflictPaths = nil
	m.snapshot = snap
	m.screen = screenConflict

	_, cmd := m.Update(syntheticKey("e"))
	if cmd != nil {
		t.Fatalf("e with no paths dispatched %T, want nil (empty conflict list is a no-op)", cmd())
	}
}

// TestEditorCommandHonoursVisualOverEditor pins the resolution
// rule: $VISUAL wins, $EDITOR is the fallback, vi is the last
// resort. Critical because git follows the same convention and
// surprising the user with a different editor between sm and git
// would be a UX paper-cut.
func TestEditorCommandHonoursVisualOverEditor(t *testing.T) {
	t.Setenv("VISUAL", "code -w")
	t.Setenv("EDITOR", "nano")
	c := editorCommand("path/to/file.go")
	if c.Args[0] != "code" {
		t.Fatalf("editor arg[0] = %q, want code (VISUAL must win)", c.Args[0])
	}
	wantArgs := []string{"code", "-w", "path/to/file.go"}
	if len(c.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", c.Args, wantArgs)
	}
	for i, w := range wantArgs {
		if c.Args[i] != w {
			t.Fatalf("args[%d] = %q, want %q", i, c.Args[i], w)
		}
	}
}

// TestEditorCommandFallsBackToEditor verifies the EDITOR fallback
// when VISUAL is unset.
func TestEditorCommandFallsBackToEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "nvim")
	c := editorCommand("file.txt")
	if c.Args[0] != "nvim" || c.Args[len(c.Args)-1] != "file.txt" {
		t.Fatalf("editor command = %v, want [nvim file.txt]", c.Args)
	}
}

// TestEditorCommandFallsBackToVi documents the last-resort default
// so a user with no editor configured still gets something usable.
func TestEditorCommandFallsBackToVi(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	c := editorCommand("file.txt")
	if c.Args[0] != "vi" {
		t.Fatalf("editor arg[0] = %q, want vi (default)", c.Args[0])
	}
}

// TestEditorFinishedSuccessTriggersRefresh pins the post-edit
// reload so a freshly-staged resolution is reflected in the
// conflict-paths list without the user having to press refresh.
func TestEditorFinishedSuccessTriggersRefresh(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	next, cmd := m.Update(editorFinishedMsg{path: "src/foo.go"})
	got := next.(Model)
	if got.lastAction != nil {
		t.Fatalf("lastAction = %+v, want nil after a successful editor exit", got.lastAction)
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload after editor exit, got nil")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg", cmd())
	}
}

// TestEditorFinishedErrorSurfacesInStatus pins the "EDITOR not
// found" path: the failure becomes a status-line entry rather than
// silently swallowing the issue.
func TestEditorFinishedErrorSurfacesInStatus(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	wantErr := errors.New("exec: \"nope\": not found")
	next, cmd := m.Update(editorFinishedMsg{path: "src/foo.go", err: wantErr})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Err != wantErr {
		t.Fatalf("lastAction.Err = %+v, want %v", got.lastAction, wantErr)
	}
	if got.lastAction.Branch != "src/foo.go" {
		t.Fatalf("lastAction.Branch = %q, want the path that failed", got.lastAction.Branch)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd on editor failure, got %T", cmd())
	}
}

// TestSnapshotClampsConflictCursorOnShrink covers the in-resolver
// refresh case: after one of N conflicts gets resolved (either via
// editor or external git operations), the cursor must land back in
// bounds rather than pointing past the end of the list.
func TestSnapshotClampsConflictCursorOnShrink(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotPaused()
	m.screen = screenConflict
	m.conflictCursor = 1 // pointing at the second of two paths
	m.firstLoad = false  // we're past the initial load

	shrunk := snapshotPaused()
	shrunk.Paused.ConflictPaths = []string{"src/foo.go"} // one less path
	next, _ := m.Update(snapshotMsg{snap: shrunk})
	got := next.(Model)
	if got.conflictCursor != 0 {
		t.Fatalf("conflictCursor = %d after shrink, want 0 (clamped to bounds)", got.conflictCursor)
	}
}

// TestViewConflictRendersHeaderAndPaths is the end-to-end
// rendering smoke: the view contains the paused branch, the
// pending queue, and every conflict path. Substring matching
// keeps trivial styling tweaks from churning the test.
func TestViewConflictRendersHeaderAndPaths(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	out := m.View()
	for _, want := range []string{
		"conflict resolver",
		"paused on feat-a",
		"via restack",
		"queue:",
		"feat-b",
		"conflicts (2):",
		"src/foo.go",
		"src/bar.go",
		"continue rebase",
		"abort rebase",
		"edit file",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// TestViewConflictWithoutPausedShowsFallback covers the routing-bug
// safety net: rendering the conflict screen without a paused
// snapshot should not panic and should tell the user how to escape.
func TestViewConflictWithoutPausedShowsFallback(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = &service.DashboardSnapshot{Trunk: "main"}
	m.screen = screenConflict

	out := m.View()
	if !strings.Contains(out, "no paused rebase") {
		t.Fatalf("View() missing fallback message. Got:\n%s", out)
	}
	if !strings.Contains(out, "back") {
		t.Fatalf("View() missing back-key hint. Got:\n%s", out)
	}
}

// TestViewConflictEmptyPathsHintsResolution covers the "all files
// staged" middle state: git has nothing to report as unmerged but
// the rebase is still paused. The view must hint that the user can
// press continue rather than silently rendering an empty section.
func TestViewConflictEmptyPathsHintsResolution(t *testing.T) {
	m := New(context.Background(), nil)
	snap := snapshotPaused()
	snap.Paused.ConflictPaths = nil
	m.snapshot = snap
	m.screen = screenConflict

	out := m.View()
	if !strings.Contains(out, "no unmerged files") {
		t.Fatalf("View() missing zero-conflicts hint. Got:\n%s", out)
	}
	if !strings.Contains(out, "press c to continue") {
		t.Fatalf("View() should suggest pressing continue. Got:\n%s", out)
	}
}

// TestRunActionCmdContinueDetectsRePaused makes sure the same
// runActionCmd plumbing the dashboard uses also handles the
// "continue → still paused on a different branch" case. Without
// this guarantee, the resolver would keep showing the original
// branch's info even after continue moved the rebase forward to a
// new branch with new conflicts.
func TestRunActionCmdContinueDetectsRePaused(t *testing.T) {
	repaused := &restack.PausedError{Branch: "feat-b"}
	cmd := runActionCmd("continue rebase", "", func() error { return repaused })
	res := cmd().(actionResult)
	if res.Paused == nil {
		t.Fatalf("Paused should be set when RestackContinue re-pauses, got nil")
	}
	if res.Paused.Branch != "feat-b" {
		t.Fatalf("re-paused branch = %q, want feat-b", res.Paused.Branch)
	}
}
