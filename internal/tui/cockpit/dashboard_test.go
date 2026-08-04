package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/service"
)

// sampleSnapshot returns a deterministic three-row stack used by
// the dashboard tests. The shape is:
//
//	main
//	├─ feat-a   ← current
//	└─ feat-b
//
// Two roots makes the cursor-bounds tests meaningful (more than
// one tracked branch) and gives the renderer ancestor-pipe logic
// to exercise.
func sampleSnapshot() *service.DashboardSnapshot {
	return &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "feat-a",
		Log: &service.LogResult{
			Trunk:   "main",
			Current: "feat-a",
			Branches: []*service.LogBranch{
				{Branch: "feat-a", Parent: "main", Depth: 1, IsCurrent: true},
				{Branch: "feat-b", Parent: "main", Depth: 1},
			},
		},
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true},
			{Branch: "feat-a", Depth: 1, IsCurrent: true},
			{Branch: "feat-b", Depth: 1, IsLastChild: true},
		},
	}
}

// TestSnapshotPositionsCursorOnCurrentOnFirstLoad pins the boot
// experience: when the user launches `sm`, the cursor lands on the
// branch HEAD is on. Without this they'd have to scroll to find
// themselves on every cockpit launch.
func TestSnapshotPositionsCursorOnCurrentOnFirstLoad(t *testing.T) {
	m := New(context.Background(), nil)
	next, _ := m.Update(snapshotMsg{snap: sampleSnapshot()})
	got := next.(Model)
	wantCursor := 1 // index of feat-a in CheckoutItems
	if got.cursor != wantCursor {
		t.Fatalf("cursor = %d, want %d (current branch row)", got.cursor, wantCursor)
	}
	if got.firstLoad {
		t.Fatalf("firstLoad still true after first snapshot")
	}
}

// TestSnapshotPreservesCursorOnRefresh guards against the bug where
// every refresh would jump the cursor back to the current branch.
// Once positioned, the user's selection sticks.
func TestSnapshotPreservesCursorOnRefresh(t *testing.T) {
	m := New(context.Background(), nil)
	// First snapshot positions cursor on feat-a.
	next, _ := m.Update(snapshotMsg{snap: sampleSnapshot()})
	got := next.(Model)
	got.cursor = 2 // user moved to feat-b

	// Second snapshot (same shape) must NOT move the cursor.
	next2, _ := got.Update(snapshotMsg{snap: sampleSnapshot()})
	got2 := next2.(Model)
	if got2.cursor != 2 {
		t.Fatalf("cursor moved on second snapshot: got %d, want 2", got2.cursor)
	}
}

// TestSnapshotClampsCursorWhenItemsShrink guards the "branch
// deleted under me" case: after a sync that prunes merged branches,
// the cursor has to land back inside the new bounds rather than
// pointing past the end.
func TestSnapshotClampsCursorWhenItemsShrink(t *testing.T) {
	m := New(context.Background(), nil)
	next, _ := m.Update(snapshotMsg{snap: sampleSnapshot()})
	got := next.(Model)
	got.cursor = 2 // feat-b row

	shrunk := &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "main",
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true},
		},
	}
	next2, _ := got.Update(snapshotMsg{snap: shrunk})
	got2 := next2.(Model)
	if got2.cursor != 0 {
		t.Fatalf("cursor = %d after shrink, want 0 (clamped to bounds)", got2.cursor)
	}
}

// TestSnapshotDispatchesDetailLoadForSelection ties two contracts
// together: a fresh snapshot positions the cursor AND kicks off a
// Service.Show fetch for the selected branch. Without the second
// half the detail pane would stay on "loading…" until the user
// moved the cursor.
func TestSnapshotDispatchesDetailLoadForSelection(t *testing.T) {
	m := New(context.Background(), service.New(""))
	_, cmd := m.Update(snapshotMsg{snap: sampleSnapshot()})
	if cmd == nil {
		t.Fatalf("Update returned nil cmd, want a detail load for the selected branch")
	}
	msg := cmd()
	dm, ok := msg.(detailMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want detailMsg", msg)
	}
	if dm.branch != "feat-a" {
		t.Fatalf("detailMsg.branch = %q, want feat-a (the current row)", dm.branch)
	}
}

// TestUpdateDashboardCursorMovesAndDispatches verifies the full
// arrow-key contract: cursor moves on Down and triggers a detail
// fetch for the new selection. Movement past the bottom is a
// no-op, not an error.
func TestUpdateDashboardCursorMovesAndDispatches(t *testing.T) {
	m := New(context.Background(), service.New(""))
	next, _ := m.Update(snapshotMsg{snap: sampleSnapshot()})
	m = next.(Model)
	// Pretend we already cached feat-a's detail so the next dispatch
	// is unambiguous about what triggered it.
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}

	next2, cmd := m.Update(syntheticKey("down"))
	got := next2.(Model)
	if got.cursor != 2 {
		t.Fatalf("cursor = %d after Down, want 2 (feat-b)", got.cursor)
	}
	if cmd == nil {
		t.Fatalf("Down should have dispatched a detail load for feat-b")
	}
	dm, ok := cmd().(detailMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want detailMsg", cmd())
	}
	if dm.branch != "feat-b" {
		t.Fatalf("dispatched detailMsg.branch = %q, want feat-b", dm.branch)
	}
}

// TestUpdateDashboardCursorBoundsAreNoOps prevents off-by-one
// regressions: pressing Up at the top or Down at the bottom must
// not move past the ends or trigger redundant detail loads.
func TestUpdateDashboardCursorBoundsAreNoOps(t *testing.T) {
	m := New(context.Background(), nil)
	next, _ := m.Update(snapshotMsg{snap: sampleSnapshot()})
	m = next.(Model)
	m.cursor = 0
	next2, cmd := m.Update(syntheticKey("up"))
	got := next2.(Model)
	if got.cursor != 0 {
		t.Fatalf("Up at cursor 0 changed it to %d, want 0", got.cursor)
	}
	if cmd != nil {
		t.Fatalf("Up at cursor 0 dispatched %T, want nil", cmd())
	}

	m.cursor = 2
	next3, cmd := m.Update(syntheticKey("down"))
	got3 := next3.(Model)
	if got3.cursor != 2 {
		t.Fatalf("Down at last row changed cursor to %d, want 2", got3.cursor)
	}
	if cmd != nil {
		t.Fatalf("Down at last row dispatched %T, want nil", cmd())
	}
}

// TestDetailMsgPopulatesCache covers the success path: an arriving
// detailMsg must land in m.details under its branch key so View can
// switch from "loading detail…" to the real BranchView.
func TestDetailMsgPopulatesCache(t *testing.T) {
	m := New(context.Background(), nil)
	view := &service.BranchView{Branch: "feat-a", Trunk: "main"}
	next, _ := m.Update(detailMsg{branch: "feat-a", view: view})
	got := next.(Model)
	if got.details["feat-a"] != view {
		t.Fatalf("details[feat-a] = %+v, want the view we sent", got.details["feat-a"])
	}
	if _, hasErr := got.detailErrs["feat-a"]; hasErr {
		t.Fatalf("detailErrs[feat-a] should be cleared after a successful load")
	}
}

// TestDetailMsgPopulatesErrorCache mirrors the success path for
// failures so the detail pane can render a clear error rather than
// an indefinite loading spinner.
func TestDetailMsgPopulatesErrorCache(t *testing.T) {
	m := New(context.Background(), nil)
	wantErr := errors.New("show failed")
	next, _ := m.Update(detailMsg{branch: "feat-a", err: wantErr})
	got := next.(Model)
	if got.detailErrs["feat-a"] != wantErr {
		t.Fatalf("detailErrs[feat-a] = %v, want %v", got.detailErrs["feat-a"], wantErr)
	}
	if _, has := got.details["feat-a"]; has {
		t.Fatalf("details[feat-a] should be cleared after a failed load")
	}
}

// TestRefreshClearsDetailCache is the contract that lets the user
// force a fresh detail read. Without it, refresh would re-load the
// snapshot but show stale per-branch data.
func TestRefreshClearsDetailCache(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}
	m.detailErrs["feat-b"] = errors.New("stale")
	m.lastAction = &actionResult{Verb: "restack", Branch: "feat-a"}

	next, _ := m.Update(syntheticKey("ctrl+r"))
	got := next.(Model)
	if len(got.details) != 0 {
		t.Fatalf("details = %+v after refresh, want empty", got.details)
	}
	if len(got.detailErrs) != 0 {
		t.Fatalf("detailErrs = %+v after refresh, want empty", got.detailErrs)
	}
	if got.lastAction != nil {
		t.Fatalf("lastAction = %+v after refresh, want nil (manual refresh resets the status banner)", got.lastAction)
	}
}

func TestSnapshotLoadErrorKeepsPreviousSnapshot(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.cursor = 1

	next, _ := m.Update(snapshotMsg{err: errors.New("git exploded")})
	got := next.(Model)
	if got.snapshot == nil {
		t.Fatalf("snapshot was cleared after refresh error")
	}
	if got.snapshot.Current != "feat-a" {
		t.Fatalf("snapshot.Current = %q, want previous snapshot to remain visible", got.snapshot.Current)
	}
	if got.loadErr == nil {
		t.Fatalf("loadErr was not recorded")
	}
}

func TestStatusRefreshMergesRowsIntoVisibleSnapshot(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.snapshot.CheckoutItems[1].PR = 10

	next, _ := m.Update(statusRefreshMsg{report: service.StatusReport{
		Branches: []service.StatusBranch{
			{
				Branch:    "feat-a",
				PR:        10,
				Checks:    gh.ChecksPass,
				Mergeable: gh.MergeMergeable,
			},
		},
	}})
	got := next.(Model)
	st := got.snapshot.GitHubStatus["feat-a"]
	if st.PR != 10 || st.Checks != gh.ChecksPass || st.Mergeable != gh.MergeMergeable {
		t.Fatalf("cached status = %+v, want merged live status for feat-a", st)
	}
}

func TestViewRendersCachedGitHubStatusAndRefreshBanner(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.snapshot.CheckoutItems[1].PR = 10
	m.snapshot.GitHubStatus = map[string]service.StatusBranch{
		"feat-a": {
			Branch:    "feat-a",
			PR:        10,
			Checks:    gh.ChecksPass,
			Mergeable: gh.MergeConflicting,
		},
	}
	m.cursor = 1
	m.statusRefreshing = true

	out := m.View()
	for _, want := range []string{
		"refreshing GitHub status",
		"CI",
		"pass",
		"conflict",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// TestViewRendersDetailFromCache is the smallest end-to-end check
// that the detail pane reads from m.details. We seed both the
// snapshot and the cache and assert real BranchView fields appear.
func TestViewRendersDetailFromCache(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.cursor = 1 // feat-a
	m.details["feat-a"] = &service.BranchView{
		Branch:       "feat-a",
		Trunk:        "main",
		Parent:       "main",
		AheadParent:  3,
		BehindParent: 0,
		AheadTrunk:   3,
		BehindTrunk:  0,
		Children:     []string{"feat-b"},
		Commits: []service.CommitView{
			{SHA: "deadbeef1234567", Subject: "feat: do the thing"},
		},
	}
	out := m.View()
	for _, want := range []string{
		"feat-a",
		"parent: main",
		"3 ahead, 0 behind",
		"children:",
		"feat-b",
		"commits (1):",
		"deadbee",
		"feat: do the thing",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// TestViewRendersDetailErrorFromCache pins the failure-pane UX:
// when Service.Show errored for the selected branch, the message
// is surfaced in the detail pane verbatim (so the user can act on
// it) rather than collapsing back to "loading…".
func TestViewRendersDetailErrorFromCache(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.cursor = 1
	m.detailErrs["feat-a"] = errors.New("not in repo")

	out := m.View()
	if !strings.Contains(out, "show feat-a failed") {
		t.Fatalf("View() missing detail-error banner. Got:\n%s", out)
	}
	if !strings.Contains(out, "not in repo") {
		t.Fatalf("View() should surface the underlying error. Got:\n%s", out)
	}
}

// TestViewEmptyStackFallsBackGracefully covers the brand-new repo
// case: only the trunk row exists, cursor pinned to it, no detail
// fetched yet. The dashboard must render *something* without
// panicking on the empty CheckoutItems slice or cursor=0 on a
// trunk-only stack.
func TestViewEmptyStackFallsBackGracefully(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "main",
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true, IsCurrent: true},
		},
	}
	out := m.View()
	if !strings.Contains(out, "main") {
		t.Fatalf("View() missing trunk row. Got:\n%s", out)
	}
}

// TestUpdateDashboardOnEmptyStackIsNoOp prevents a crash if a key
// arrives between launch and the first snapshot load.
func TestUpdateDashboardOnEmptyStackIsNoOp(t *testing.T) {
	m := New(context.Background(), nil)
	if _, cmd := m.Update(syntheticKey("down")); cmd != nil {
		// We accept a non-nil cmd only if it is a tea.Quit or similar;
		// nav keys must not dispatch anything when there's nothing to
		// navigate.
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("Down should not produce a quit message")
		}
		t.Fatalf("Down on empty stack dispatched a cmd, want nil")
	}
}
