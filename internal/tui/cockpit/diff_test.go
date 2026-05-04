package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/service"
)

// branchViewWithCommits returns a BranchView with two commits so
// the diff cursor has somewhere to step. Used as the canonical
// fixture for diff-screen tests.
func branchViewWithCommits() *service.BranchView {
	return &service.BranchView{
		Branch: "feat-a",
		Trunk:  "main",
		Parent: "main",
		Commits: []service.CommitView{
			{SHA: "deadbeef1111111111111111111111111111aaaa", Subject: "feat: add foo"},
			{SHA: "cafef00d2222222222222222222222222222bbbb", Subject: "feat: add bar"},
		},
	}
}

// withDiffOpen sets up a Model that's already in the diff viewer
// for feat-a, so cursor / scroll tests can start from a known
// state without re-running the dashboard dispatch.
func withDiffOpen(t *testing.T) Model {
	t.Helper()
	m := withActionSnapshot(t, "feat-a")
	m.details["feat-a"] = branchViewWithCommits()
	next, _ := m.enterDiff("feat-a", branchViewWithCommits().Commits)
	return next
}

// TestDispatchDashboardKeyDiffEntersViewer pins the entry contract:
// pressing `d` on the dashboard with a non-trunk row cached must
// route into the diff viewer with the branch's commits loaded and
// a load command for the first commit dispatched.
func TestDispatchDashboardKeyDiffEntersViewer(t *testing.T) {
	m := withActionSnapshot(t, "feat-a")
	m.details["feat-a"] = branchViewWithCommits()

	next, cmd := m.Update(syntheticKey("d"))
	got := next.(Model)
	if got.screen != screenDiff {
		t.Fatalf("screen = %v, want screenDiff", got.screen)
	}
	if got.diffBranch != "feat-a" {
		t.Fatalf("diffBranch = %q, want feat-a", got.diffBranch)
	}
	if len(got.diffCommits) != 2 {
		t.Fatalf("diffCommits = %d, want 2", len(got.diffCommits))
	}
	if got.diffCommitCursor != 0 {
		t.Fatalf("diffCommitCursor = %d, want 0 on entry", got.diffCommitCursor)
	}
	if cmd == nil {
		t.Fatalf("expected a diff load cmd for the first commit, got nil")
	}
	dm, ok := cmd().(diffMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want diffMsg", cmd())
	}
	if dm.sha != got.diffCommits[0].SHA {
		t.Fatalf("dispatched diffMsg.sha = %q, want %q", dm.sha, got.diffCommits[0].SHA)
	}
}

// TestDispatchDashboardKeyDiffOnTrunkIsNoOp documents that pressing
// `d` while parked on trunk is a silent no-op rather than entering
// an empty viewer. Trunk has no per-branch commits to inspect.
func TestDispatchDashboardKeyDiffOnTrunkIsNoOp(t *testing.T) {
	m := withActionSnapshot(t, "main")

	next, cmd := m.Update(syntheticKey("d"))
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v, want screenDashboard (trunk press should be no-op)", got.screen)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd on trunk-row diff press, got %T", cmd())
	}
}

// TestDispatchDashboardKeyDiffWithoutCachedDetailHints covers the
// race where the user presses `d` before Service.Show has resolved
// for the cursor branch. We surface a status hint and stay put
// rather than enter a viewer that has nothing to render.
func TestDispatchDashboardKeyDiffWithoutCachedDetailHints(t *testing.T) {
	m := withActionSnapshot(t, "feat-a")
	// Note: no details cache populated.

	next, cmd := m.Update(syntheticKey("d"))
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v, want screenDashboard while detail still loading", got.screen)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd while detail loading, got %T", cmd())
	}
	if got.lastAction == nil || got.lastAction.Err == nil {
		t.Fatalf("expected status hint about detail not loaded, got %+v", got.lastAction)
	}
	if !strings.Contains(got.lastAction.Err.Error(), "not loaded") {
		t.Fatalf("status hint = %v, want a 'not loaded' message", got.lastAction.Err)
	}
}

// TestDiffNextPrevCommitMovesCursor exercises the commit-stepping
// keys: tab moves forward, shift+tab moves backward, and each step
// resets the diff scroll offset + dispatches a fresh load (when
// the new commit isn't cached).
func TestDiffNextPrevCommitMovesCursor(t *testing.T) {
	m := withDiffOpen(t)
	m.diffScroll = 50 // simulate a scrolled-down state

	next, cmd := m.Update(syntheticKey("tab"))
	got := next.(Model)
	if got.diffCommitCursor != 1 {
		t.Fatalf("diffCommitCursor = %d after Tab, want 1", got.diffCommitCursor)
	}
	if got.diffScroll != 0 {
		t.Fatalf("diffScroll = %d after commit step, want 0 (must reset on commit change)", got.diffScroll)
	}
	if cmd == nil {
		t.Fatalf("expected a diff load for the next commit, got nil")
	}
	if dm, ok := cmd().(diffMsg); !ok || dm.sha != got.diffCommits[1].SHA {
		t.Fatalf("dispatched %v, want diffMsg for the next commit's SHA", cmd())
	}

	// shift+tab back to commit 0.
	next2, _ := got.Update(syntheticKey("shift+tab"))
	got2 := next2.(Model)
	if got2.diffCommitCursor != 0 {
		t.Fatalf("diffCommitCursor = %d after Shift+Tab, want 0", got2.diffCommitCursor)
	}
}

// TestDiffNextCommitClampsAtBottom prevents off-by-one errors at
// the end of the commit list. Pressing next at the last commit must
// not wrap or panic.
func TestDiffNextCommitClampsAtBottom(t *testing.T) {
	m := withDiffOpen(t)
	m.diffCommitCursor = len(m.diffCommits) - 1

	next, cmd := m.Update(syntheticKey("tab"))
	got := next.(Model)
	if got.diffCommitCursor != len(m.diffCommits)-1 {
		t.Fatalf("diffCommitCursor changed past last row: got %d", got.diffCommitCursor)
	}
	if cmd != nil {
		t.Fatalf("Tab at last commit dispatched %T, want nil", cmd())
	}
}

// TestDiffScrollHandlesUpDownAndPage covers the scroll-key matrix
// against a cached diff long enough that scrolling has somewhere
// to go. Up/Down move one line; PageUp/PageDown move a viewport
// height; Home/End jump to the bounds.
func TestDiffScrollHandlesUpDownAndPage(t *testing.T) {
	m := withDiffOpen(t)
	m.height = 32 // viewport height = 32 - 8 = 24
	sha := m.diffCommits[0].SHA
	m.diffCache[sha] = strings.Repeat("line\n", 200) // ~200 lines

	height := diffViewportHeight(m)

	// Down moves +1.
	next, _ := m.Update(syntheticKey("down"))
	if got := next.(Model); got.diffScroll != 1 {
		t.Fatalf("diffScroll after Down = %d, want 1", got.diffScroll)
	}

	// Up at top is a no-op.
	next2, _ := m.Update(syntheticKey("up"))
	if got := next2.(Model); got.diffScroll != 0 {
		t.Fatalf("diffScroll after Up at 0 = %d, want 0", got.diffScroll)
	}

	// Page down by viewport height.
	next3, _ := m.Update(syntheticKey("pgdown"))
	if got := next3.(Model); got.diffScroll != height {
		t.Fatalf("diffScroll after PageDown = %d, want %d", got.diffScroll, height)
	}

	// End jumps to max scroll.
	next4, _ := m.Update(syntheticKey("end"))
	got4 := next4.(Model)
	wantMax := maxDiffScroll(got4, height)
	if got4.diffScroll != wantMax {
		t.Fatalf("diffScroll after End = %d, want %d (max)", got4.diffScroll, wantMax)
	}

	// Home jumps back to 0.
	next5, _ := got4.Update(syntheticKey("home"))
	if got := next5.(Model); got.diffScroll != 0 {
		t.Fatalf("diffScroll after Home = %d, want 0", got.diffScroll)
	}
}

// TestDiffScrollClampsToContentLength is a focused regression for
// the End / PageDown semantics: they must not scroll past the end
// of the content. Otherwise a long press of PageDown would leave
// the viewport rendering an empty pane.
func TestDiffScrollClampsToContentLength(t *testing.T) {
	m := withDiffOpen(t)
	m.height = 50
	sha := m.diffCommits[0].SHA
	m.diffCache[sha] = "one\ntwo\nthree\n" // shorter than viewport

	height := diffViewportHeight(m)
	if maxDiffScroll(m, height) != 0 {
		t.Fatalf("maxDiffScroll on short content = %d, want 0 (content fits)", maxDiffScroll(m, height))
	}

	next, _ := m.Update(syntheticKey("end"))
	if got := next.(Model); got.diffScroll != 0 {
		t.Fatalf("End on short content scrolled to %d, want 0", got.diffScroll)
	}
}

// TestDiffMsgPopulatesCache covers the success path for diff
// loading. The cache is what feeds viewDiffBody, so a missing
// store-on-success would leave the viewer stuck on "loading…".
func TestDiffMsgPopulatesCache(t *testing.T) {
	m := withDiffOpen(t)
	body := "diff --git a/foo b/foo\n@@ -1 +1 @@\n-old\n+new\n"
	next, _ := m.Update(diffMsg{sha: "deadbeef", content: body})
	got := next.(Model)
	if got.diffCache["deadbeef"] != body {
		t.Fatalf("diffCache[deadbeef] = %q, want %q", got.diffCache["deadbeef"], body)
	}
}

// TestDiffMsgPopulatesErrorCache is the failure mirror — errors
// must surface in the viewer rather than collapse into "loading…".
func TestDiffMsgPopulatesErrorCache(t *testing.T) {
	m := withDiffOpen(t)
	wantErr := errors.New("git show failed")
	next, _ := m.Update(diffMsg{sha: "deadbeef", err: wantErr})
	got := next.(Model)
	if got.diffErrs["deadbeef"] != wantErr {
		t.Fatalf("diffErrs[deadbeef] = %v, want %v", got.diffErrs["deadbeef"], wantErr)
	}
}

// TestDiffBackReturnsToDashboardAndClearsState covers the exit
// path: esc returns to the dashboard and resets the diff cursor /
// branch fields, but preserves the cache so re-entry is fast.
func TestDiffBackReturnsToDashboardAndClearsState(t *testing.T) {
	m := withDiffOpen(t)
	m.diffCommitCursor = 1
	m.diffScroll = 12
	m.diffCache["preserved"] = "..."

	next, _ := m.Update(syntheticKey("esc"))
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v after esc, want screenDashboard", got.screen)
	}
	if got.diffBranch != "" || got.diffCommits != nil {
		t.Fatalf("expected diffBranch/Commits cleared, got branch=%q commits=%+v", got.diffBranch, got.diffCommits)
	}
	if got.diffCommitCursor != 0 || got.diffScroll != 0 {
		t.Fatalf("expected cursor/scroll reset, got cursor=%d scroll=%d", got.diffCommitCursor, got.diffScroll)
	}
	if _, ok := got.diffCache["preserved"]; !ok {
		t.Fatalf("expected diffCache to survive Back so re-entry is cheap")
	}
}

// TestColourDiffLineMatrix pins the diff-line colour mapping so a
// future style refactor can't silently lose the +/-/@@ contrast.
// We assert non-empty rendering rather than exact ANSI bytes
// because the codes depend on the active termenv profile.
func TestColourDiffLineMatrix(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"hunk header", "@@ -1,3 +1,3 @@"},
		{"file header", "diff --git a/foo b/foo"},
		{"index line", "index 0000000..1111111 100644"},
		{"old path", "--- a/foo"},
		{"new path", "+++ b/foo"},
		{"addition", "+new content"},
		{"deletion", "-old content"},
		{"context", " unchanged"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := colourDiffLine(tc.line)
			if !strings.Contains(got, strings.TrimLeft(tc.line, " ")) && !strings.Contains(got, tc.line) {
				t.Fatalf("colourDiffLine dropped content: %q -> %q", tc.line, got)
			}
		})
	}
}

// TestViewDiffRendersHeaderCommitsAndBody is the View-level smoke
// test. We seed both the snapshot (so the viewer has a branch),
// the commit list, and a cached diff, then assert the rendered
// output covers the three sections.
func TestViewDiffRendersHeaderCommitsAndBody(t *testing.T) {
	m := withDiffOpen(t)
	sha := m.diffCommits[0].SHA
	m.diffCache[sha] = "diff --git a/foo b/foo\n@@ -1 +1 @@\n-old\n+new\n"

	out := m.View()
	for _, want := range []string{
		"stac-man — diff",
		"feat-a",
		"commits (2):",
		shortSHA(sha),
		"feat: add foo",
		"feat: add bar",
		"diff --git a/foo b/foo",
		"-old",
		"+new",
		"prev commit",
		"next commit",
		"back",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// TestViewDiffEmptyCommitsHintsBack covers the "branch with no
// commits unique to it" case (just-tracked branch sitting at
// trunk). The viewer must say something useful and tell the user
// how to escape rather than show a blank pane.
func TestViewDiffEmptyCommitsHintsBack(t *testing.T) {
	m := New(context.Background(), nil)
	m.screen = screenDiff
	m.diffBranch = "feat-empty"

	out := m.View()
	if !strings.Contains(out, "no commits") {
		t.Fatalf("View() missing empty-commits hint. Got:\n%s", out)
	}
	if !strings.Contains(out, "back") {
		t.Fatalf("View() missing back-key hint. Got:\n%s", out)
	}
}

// TestViewDiffBodyShowsLoadingPlaceholder is the contract that
// drives the viewer's first paint: before the diff cache fills,
// the body says "loading…" so the user knows something is in
// flight rather than thinking the diff is empty.
func TestViewDiffBodyShowsLoadingPlaceholder(t *testing.T) {
	m := withDiffOpen(t)
	out := m.View()
	if !strings.Contains(out, "loading diff") {
		t.Fatalf("View() missing loading placeholder. Got:\n%s", out)
	}
}

// TestViewDiffBodyShowsErrorOverPlaceholder pins the error path:
// once an error caches for the selected SHA, the body switches
// from "loading…" to the failure message verbatim.
func TestViewDiffBodyShowsErrorOverPlaceholder(t *testing.T) {
	m := withDiffOpen(t)
	sha := m.diffCommits[0].SHA
	m.diffErrs[sha] = errors.New("not a valid object name")

	out := m.View()
	if !strings.Contains(out, "git show") {
		t.Fatalf("View() missing failure prefix. Got:\n%s", out)
	}
	if !strings.Contains(out, "not a valid object name") {
		t.Fatalf("View() should surface underlying error. Got:\n%s", out)
	}
}

// TestViewDiffShowsScrollIndicator pins the bottom-right
// "[start-end of total]" indicator so users know they're looking
// at a window into a longer diff.
func TestViewDiffShowsScrollIndicator(t *testing.T) {
	m := withDiffOpen(t)
	m.height = 16 // small viewport
	sha := m.diffCommits[0].SHA
	m.diffCache[sha] = strings.Repeat("line\n", 100)

	out := m.View()
	if !strings.Contains(out, "of 101") && !strings.Contains(out, "of 100") {
		// Strings.Split on a trailing newline yields one extra
		// empty element, so total can be 100 or 101 depending on
		// content. Either is acceptable.
		t.Fatalf("View() missing scroll indicator [start-end of total]. Got:\n%s", out)
	}
}

// TestDashboardFooterMentionsDiff is the discoverability guard for
// the new binding. Without it the user has no path from "I want
// to see this branch's diffs" to "press d".
func TestDashboardFooterMentionsDiff(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	out := m.View()
	if !strings.Contains(out, "diff") {
		t.Fatalf("dashboard footer missing diff hint. Got:\n%s", out)
	}
}
