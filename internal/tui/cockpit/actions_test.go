package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/service"
)

// snapshotForActions returns a deterministic stack used by the
// action dispatch tests. Trunk row at index 0 lets the trunk-guard
// tests target a known position.
//
//	main             ← cursor on trunk
//	├─ feat-a    ← current
//	└─ feat-b
func snapshotForActions() *service.DashboardSnapshot {
	return &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "feat-a",
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true},
			{Branch: "feat-a", Depth: 1, IsCurrent: true},
			{Branch: "feat-b", Depth: 1, IsLastChild: true},
		},
	}
}

// withActionSnapshot returns a Model preloaded with the
// snapshotForActions stack and the cursor on `branch` so each
// action test starts from a known state.
func withActionSnapshot(t *testing.T, branch string) Model {
	t.Helper()
	m := New(context.Background(), service.New(""))
	m.snapshot = snapshotForActions()
	m.firstLoad = false
	for i, it := range m.snapshot.CheckoutItems {
		if it.Branch == branch {
			m.cursor = i
			return m
		}
	}
	t.Fatalf("branch %q not in snapshotForActions", branch)
	return m
}

// TestActionDispatchTable covers every cursor- and HEAD-targeted
// action binding in one shot. We assert the *dispatch shape* — the
// returned cmd resolves to an actionResult with the expected verb
// and target branch — rather than the underlying service outcome
// (which depends on the test cwd and is exercised by service-layer
// tests). This is the same trade-off TestUpdateRefreshDispatchesLoad
// makes for refresh.
func TestActionDispatchTable(t *testing.T) {
	cases := []struct {
		name       string
		key        string
		cursorOn   string
		wantVerb   func(Keymap) string
		wantBranch string
	}{
		{
			name:       "checkout dispatches with cursor branch",
			key:        "enter",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Checkout.Help },
			wantBranch: "feat-b",
		},
		{
			name:       "restack dispatches with cursor branch",
			key:        "r",
			cursorOn:   "feat-a",
			wantVerb:   func(k Keymap) string { return k.Restack.Help },
			wantBranch: "feat-a",
		},
		{
			name:       "track dispatches with cursor branch",
			key:        "t",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Track.Help },
			wantBranch: "feat-b",
		},
		{
			name:       "untrack dispatches with cursor branch",
			key:        "T",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Untrack.Help },
			wantBranch: "feat-b",
		},
		{
			name:       "modify dispatches without a branch operand",
			key:        "m",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Modify.Help },
			wantBranch: "",
		},
		{
			name:       "fold dispatches without a branch operand",
			key:        "f",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Fold.Help },
			wantBranch: "",
		},
		{
			name:       "absorb dispatches without a branch operand",
			key:        "a",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Absorb.Help },
			wantBranch: "",
		},
		{
			name:       "undo dispatches without a branch operand",
			key:        "u",
			cursorOn:   "feat-b",
			wantVerb:   func(k Keymap) string { return k.Undo.Help },
			wantBranch: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := withActionSnapshot(t, tc.cursorOn)
			_, cmd := m.Update(syntheticKey(tc.key))
			if cmd == nil {
				t.Fatalf("Update on %q returned nil cmd, want an action dispatch", tc.key)
			}
			res, ok := cmd().(actionResult)
			if !ok {
				t.Fatalf("cmd() = %T, want actionResult", cmd())
			}
			wantVerb := tc.wantVerb(m.keys)
			if res.Verb != wantVerb {
				t.Fatalf("Verb = %q, want %q", res.Verb, wantVerb)
			}
			if res.Branch != tc.wantBranch {
				t.Fatalf("Branch = %q, want %q", res.Branch, tc.wantBranch)
			}
		})
	}
}

// TestActionTrunkGuards documents which mutating actions silently
// no-op when the cursor is parked on the trunk row. Restack /
// track / untrack make no sense on trunk and the service layer
// would surface a noisy error; the cockpit shadows that here so a
// fat-fingered keypress is harmless.
//
// Checkout is intentionally NOT in this table: switching to trunk
// from anywhere is a routine workflow and must keep working.
func TestActionTrunkGuards(t *testing.T) {
	for _, key := range []string{"r", "t", "T"} {
		t.Run(key, func(t *testing.T) {
			m := withActionSnapshot(t, "main")
			_, cmd := m.Update(syntheticKey(key))
			if cmd != nil {
				t.Fatalf("Update on %q with cursor on trunk dispatched %T, want nil", key, cmd())
			}
		})
	}
}

// TestCheckoutOnTrunkStillDispatches is the inverse of
// TestActionTrunkGuards for the one action that *should* work on
// trunk. Without this, a refactor that over-broadens the trunk
// guard would silently break "Enter to switch back to main".
func TestCheckoutOnTrunkStillDispatches(t *testing.T) {
	m := withActionSnapshot(t, "main")
	_, cmd := m.Update(syntheticKey("enter"))
	if cmd == nil {
		t.Fatalf("Enter on trunk row returned nil cmd, want a checkout dispatch")
	}
	res, ok := cmd().(actionResult)
	if !ok {
		t.Fatalf("cmd() = %T, want actionResult", cmd())
	}
	if res.Branch != "main" {
		t.Fatalf("Branch = %q, want main", res.Branch)
	}
}

// TestActionResultSuccessTriggersRefresh covers the post-action
// reload contract: a successful actionResult invalidates the
// detail caches and dispatches a fresh snapshotMsg. Without this
// the dashboard would render stale data after every mutation.
func TestActionResultSuccessTriggersRefresh(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}
	m.detailErrs["feat-b"] = errors.New("stale")

	next, cmd := m.Update(actionResult{Verb: "restack", Branch: "feat-a"})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Verb != "restack" {
		t.Fatalf("lastAction = %+v, want restack outcome stored", got.lastAction)
	}
	if len(got.details) != 0 || len(got.detailErrs) != 0 {
		t.Fatalf("expected detail caches cleared on success, got details=%+v errs=%+v", got.details, got.detailErrs)
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload command after success, got nil")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg", cmd())
	}
}

// TestActionResultFailureKeepsState covers the failure-mode
// contract: a plain (non-paused) failure stores the result for the
// status line but does NOT trigger a refresh. Reloading on every
// failure would mask the user's mental model of what was
// in-progress when the error landed.
func TestActionResultFailureKeepsState(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}

	wantErr := errors.New("git rebase failed")
	next, cmd := m.Update(actionResult{Verb: "restack", Branch: "feat-a", Err: wantErr})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Err != wantErr {
		t.Fatalf("lastAction.Err = %v, want %v", got.lastAction, wantErr)
	}
	if cmd != nil {
		t.Fatalf("expected nil cmd on failure, got %T", cmd())
	}
	if got.details["feat-a"] == nil {
		t.Fatalf("details cache should be preserved on failure so the user keeps context")
	}
}

// TestActionResultPausedTriggersRefreshAndPreservesError covers
// the paused-rebase contract: paused outcomes still flush caches
// and reload (so the snapshot's Paused field populates) AND keep
// the *PausedError on lastAction so the status line / future
// conflict resolver can read it.
func TestActionResultPausedTriggersRefreshAndPreservesError(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}

	paused := &service.PausedError{Branch: "feat-a", Wrapped: errors.New("conflict")}
	next, cmd := m.Update(actionResult{Verb: "restack", Branch: "feat-a", Err: paused, Paused: paused})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Paused != paused {
		t.Fatalf("lastAction.Paused not stored, got %+v", got.lastAction)
	}
	if len(got.details) != 0 {
		t.Fatalf("expected detail cache cleared on paused, got %+v", got.details)
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload after paused so banner updates")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg", cmd())
	}
}

// TestRunActionCmdDetectsPausedError pins the paused-detection
// plumbing for the helper itself so per-action helpers can delegate
// without re-implementing errors.As. Catches the bug where a future
// service-layer wrapper changes the error chain shape.
func TestRunActionCmdDetectsPausedError(t *testing.T) {
	paused := &restack.PausedError{Branch: "feat-a"}
	cmd := runActionCmd("restack", "feat-a", func() error { return paused })
	res := cmd().(actionResult)
	if res.Paused == nil {
		t.Fatalf("Paused should be set when fn returns *PausedError, got nil")
	}
	if res.Paused != paused {
		t.Fatalf("Paused = %p, want the same pointer the action returned (%p)", res.Paused, paused)
	}
	if res.Err != paused {
		t.Fatalf("Err = %v, want the original error preserved alongside Paused", res.Err)
	}
}

// TestStatusLineFormats locks down the three status-line shapes so
// tests reading the rendered View can rely on stable substrings.
// Touching the format means updating this test, which is the
// intended forcing function.
func TestStatusLineFormats(t *testing.T) {
	cases := []struct {
		name string
		res  *actionResult
		want string
	}{
		{
			name: "nil result renders nothing",
			res:  nil,
			want: "",
		},
		{
			name: "success with branch",
			res:  &actionResult{Verb: "restack", Branch: "feat-a"},
			want: "restack feat-a ✓",
		},
		{
			name: "success without branch",
			res:  &actionResult{Verb: "amend"},
			want: "amend ✓",
		},
		{
			name: "failure with branch surfaces error text",
			res:  &actionResult{Verb: "restack", Branch: "feat-a", Err: errors.New("boom")},
			want: "restack feat-a failed: boom",
		},
		{
			name: "failure without branch surfaces error text",
			res:  &actionResult{Verb: "amend", Err: errors.New("nothing staged")},
			want: "amend failed: nothing staged",
		},
		{
			name: "paused surfaces verb and branch",
			res: &actionResult{
				Verb:   "restack",
				Branch: "feat-a",
				Err:    &restack.PausedError{Branch: "feat-a"},
				Paused: &restack.PausedError{Branch: "feat-a"},
			},
			want: "paused: restack on feat-a",
		},
		{
			name: "paused without branch falls back to PausedError.Branch",
			res: &actionResult{
				Verb:   "amend",
				Err:    &restack.PausedError{Branch: "feat-c"},
				Paused: &restack.PausedError{Branch: "feat-c"},
			},
			want: "paused: amend on feat-c",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := statusLine(tc.res)
			if tc.want == "" && got != "" {
				t.Fatalf("statusLine = %q, want empty", got)
			}
			if !strings.Contains(got, tc.want) {
				t.Fatalf("statusLine = %q, want substring %q", got, tc.want)
			}
		})
	}
}

// TestViewRendersActionStatus is the View-level smoke test for the
// status banner. Without it a refactor that drops the status pane
// from viewDashboard wouldn't be caught — the per-helper tests
// above only cover the model side.
func TestViewRendersActionStatus(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = snapshotForActions()
	m.cursor = 1
	m.lastAction = &actionResult{Verb: "restack", Branch: "feat-a"}

	out := m.View()
	if !strings.Contains(out, "restack feat-a ✓") {
		t.Fatalf("View() missing action status. Got:\n%s", out)
	}
}

// TestViewActionStatusSuppressedWhenNil pins the inverse: the
// status pane disappears when there's nothing to show, so the
// dashboard doesn't carry an empty styled line every paint.
func TestViewActionStatusSuppressedWhenNil(t *testing.T) {
	if got := viewActionStatus(nil); got != "" {
		t.Fatalf("viewActionStatus(nil) = %q, want empty", got)
	}
}
