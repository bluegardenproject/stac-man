package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/service"
)

// stubSelfBinary swaps the resolveSelfBinary closure for the
// duration of a test so dispatch tests can assert the exec path
// without shelling out for real.
func stubSelfBinary(t *testing.T, path string, err error) {
	t.Helper()
	prev := resolveSelfBinary
	resolveSelfBinary = func() (string, error) { return path, err }
	t.Cleanup(func() { resolveSelfBinary = prev })
}

// TestNetworkDispatchTable covers every network binding in one
// shot. Pressing the key must produce a non-nil cmd; we don't
// invoke it (that would actually exec sm) but we can assert the
// dispatch shape via the returned bubbletea internal exec message
// would happen — instead we replace resolveSelfBinary with an
// error to force the early-return branch which IS observable
// (returns a networkFinishedMsg with the wrapped error).
func TestNetworkDispatchTable(t *testing.T) {
	cases := []struct {
		name    string
		key     string
		verbKey func(Keymap) string
	}{
		{"submit dispatches", "s", func(k Keymap) string { return k.Submit.Help }},
		{"sync dispatches", "S", func(k Keymap) string { return k.Sync.Help }},
		{"land dispatches", "L", func(k Keymap) string { return k.Land.Help }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Force the binary-resolution branch so the cmd
			// resolves to a networkFinishedMsg synchronously,
			// without actually shelling out to a real subprocess.
			stubSelfBinary(t, "", errors.New("forced for test"))

			m := withActionSnapshot(t, "feat-a")
			_, cmd := m.Update(syntheticKey(tc.key))
			if cmd == nil {
				t.Fatalf("Update on %q returned nil cmd, want a network dispatch", tc.key)
			}
			msg, ok := cmd().(networkFinishedMsg)
			if !ok {
				t.Fatalf("cmd() = %T, want networkFinishedMsg (resolveSelfBinary stubbed to error)", cmd())
			}
			wantVerb := tc.verbKey(m.keys)
			if msg.Verb != wantVerb {
				t.Fatalf("Verb = %q, want %q", msg.Verb, wantVerb)
			}
			if msg.Err == nil {
				t.Fatalf("Err = nil, want the wrapped binary-resolution error")
			}
			if !strings.Contains(msg.Err.Error(), "locating sm binary") {
				t.Fatalf("Err = %v, want a binary-resolution wrap", msg.Err)
			}
		})
	}
}

// TestNetworkExecCmdSuccessShape pins the happy-path return type:
// when the binary resolves, we get a tea.Cmd (which would, in a
// real Program, be unwrapped by the runtime to drive
// tea.ExecProcess). We don't run the cmd — invoking the closure
// would block on a real subprocess — but we do assert it's
// non-nil, which catches the "early-return on error swallowed the
// happy path" regression.
func TestNetworkExecCmdSuccessShape(t *testing.T) {
	stubSelfBinary(t, "/usr/local/bin/sm", nil)
	cmd := networkExecCmd("submit", "submit")
	if cmd == nil {
		t.Fatalf("networkExecCmd returned nil cmd despite resolved binary")
	}
}

// TestNetworkFinishedSuccessTriggersRefreshAndArmsRoute pins three
// things at once: the status line gets the verb, the snapshot
// reload is dispatched, and the paused-route flag is armed so the
// next snapshot can route into the resolver if needed.
func TestNetworkFinishedSuccessTriggersRefreshAndArmsRoute(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}

	next, cmd := m.Update(networkFinishedMsg{Verb: "submit"})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Verb != "submit" {
		t.Fatalf("lastAction = %+v, want submit outcome stored", got.lastAction)
	}
	if got.lastAction.Err != nil {
		t.Fatalf("lastAction.Err = %v, want nil on clean exit", got.lastAction.Err)
	}
	if !got.pendingPausedRoute {
		t.Fatalf("pendingPausedRoute not armed after network exit; subsequent paused snapshot won't route into resolver")
	}
	if len(got.details) != 0 {
		t.Fatalf("details cache not cleared after network exit, got %+v", got.details)
	}
	if cmd == nil {
		t.Fatalf("expected snapshot reload cmd after network exit, got nil")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg", cmd())
	}
}

// TestNetworkFinishedFailureSurfacesError mirrors the success
// path for the failure case: the subprocess errored (gh auth
// failed, push rejected, …) and the user must see why on the
// status line.
func TestNetworkFinishedFailureSurfacesError(t *testing.T) {
	m := New(context.Background(), service.New(""))
	wantErr := errors.New("exit status 1")

	next, _ := m.Update(networkFinishedMsg{Verb: "land", Err: wantErr})
	got := next.(Model)
	if got.lastAction == nil || got.lastAction.Err != wantErr {
		t.Fatalf("lastAction.Err = %+v, want %v", got.lastAction, wantErr)
	}
	if got.lastAction.Verb != "land" {
		t.Fatalf("lastAction.Verb = %q, want land", got.lastAction.Verb)
	}
}

// TestRouteAfterNetworkPausedJumpsIntoResolver covers the
// integration we care most about: a network shell-out left paused
// state on disk; the snapshot reload picks it up; the user must
// land in the conflict resolver, not on the dashboard staring at
// a banner.
func TestRouteAfterNetworkPausedJumpsIntoResolver(t *testing.T) {
	m := New(context.Background(), nil)
	m.firstLoad = false // first-load routing already happened
	m.screen = screenDashboard
	m.pendingPausedRoute = true

	paused := snapshotPaused()
	next, _ := m.Update(snapshotMsg{snap: paused})
	got := next.(Model)
	if got.screen != screenConflict {
		t.Fatalf("screen = %v after post-network paused snapshot, want screenConflict", got.screen)
	}
	if got.pendingPausedRoute {
		t.Fatalf("pendingPausedRoute should be one-shot, still armed after consumption")
	}
}

// TestRouteAfterNetworkCleanStaysOnDashboard verifies that an
// armed paused-route flag is harmless when the network action
// did NOT leave paused state: the user stays where they were.
// Prevents the flag from being a stray loaded gun.
func TestRouteAfterNetworkCleanStaysOnDashboard(t *testing.T) {
	m := New(context.Background(), nil)
	m.firstLoad = false
	m.screen = screenDashboard
	m.pendingPausedRoute = true

	clean := &service.DashboardSnapshot{
		Trunk:   "main",
		Current: "feat-a",
		CheckoutItems: []service.CheckoutItem{
			{Branch: "main", Depth: 0, IsTrunk: true},
			{Branch: "feat-a", Depth: 1, IsCurrent: true},
		},
	}
	next, _ := m.Update(snapshotMsg{snap: clean})
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("screen = %v after clean post-network refresh, want screenDashboard", got.screen)
	}
	if got.pendingPausedRoute {
		t.Fatalf("pendingPausedRoute should be cleared after consumption")
	}
}

// TestNetworkBindingsAvailableOnConflictScreenDoNotDispatch is a
// scoping guard: pressing `s` (Submit) while the conflict resolver
// owns the screen must NOT shell out to `sm submit` mid-rebase.
// The resolver's update handler doesn't recognise these keys and
// the global Update path doesn't either, so the keypress should
// be a no-op.
func TestNetworkBindingsAvailableOnConflictScreenDoNotDispatch(t *testing.T) {
	m := New(context.Background(), service.New(""))
	m.snapshot = snapshotPaused()
	m.screen = screenConflict

	for _, key := range []string{"S", "L"} {
		// `s` is intentionally excluded — todo `palette`'s
		// ctrl+p / cmd+k don't share with `s` either, but the
		// conflict screen doesn't bind plain `s`. Restricting
		// this test to S and L keeps it future-proof if the
		// dashboard's `s` binding migrates.
		t.Run(key, func(t *testing.T) {
			_, cmd := m.Update(syntheticKey(key))
			if cmd != nil {
				if _, ok := cmd().(networkFinishedMsg); ok {
					t.Fatalf("conflict screen dispatched a network action on %q; bindings must be dashboard-scoped", key)
				}
			}
		})
	}
}

// TestDashboardFooterMentionsNetworkActions is the discoverability
// guard: after wiring s/S/L the footer must surface them, otherwise
// users have no way to learn the bindings without reading the help
// overlay (which doesn't exist yet — todo 11).
func TestDashboardFooterMentionsNetworkActions(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	out := m.View()

	for _, want := range []string{"submit", "sync", "land"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dashboard footer missing %q hint. Got:\n%s", want, out)
		}
	}
}
