package cockpit

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

// TestNewStartsOnDashboard pins the boot contract: a fresh model
// renders the dashboard, not some half-initialised "no screen"
// fallback. If a future refactor accidentally changes the zero
// value of `screen`, this catches it before users see "(cockpit:
// screen N not yet implemented)".
func TestNewStartsOnDashboard(t *testing.T) {
	m := New(context.Background(), nil)
	if m.screen != screenDashboard {
		t.Fatalf("New().screen = %v, want screenDashboard", m.screen)
	}
	if m.snapshot != nil {
		t.Fatalf("expected nil snapshot before Init runs, got %+v", m.snapshot)
	}
}

// TestUpdateQuitKeyMatrix locks down every binding in DefaultKeymap
// for Quit. Adding a new quit synonym must keep tea.Quit firing;
// removing one must surface here as a failure rather than leave a
// dead key in the help overlay.
func TestUpdateQuitKeyMatrix(t *testing.T) {
	m := New(context.Background(), nil)
	for _, key := range m.keys.Quit.Keys {
		t.Run(key, func(t *testing.T) {
			next, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(key)}))
			// Some quit synonyms are special keys (ctrl+c) rather than
			// runes — fall back to a synthetic key whose String() matches.
			if cmd == nil {
				next, cmd = m.Update(syntheticKey(key))
			}
			model, ok := next.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", next)
			}
			if !model.quitting {
				t.Fatalf("quitting = false after pressing %q, want true", key)
			}
			if cmd == nil {
				t.Fatalf("Update returned nil cmd for quit key %q, want tea.Quit", key)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatalf("cmd() returned %T, want tea.QuitMsg for quit key %q", cmd(), key)
			}
		})
	}
}

// TestUpdateRefreshDispatchesLoad pins the contract that pressing
// the Refresh binding schedules a snapshot reload — without it the
// dashboard would go stale silently after any in-process op. We
// only assert the dispatch shape (cmd returns a snapshotMsg);
// whether the underlying Snapshot succeeds depends on the test's
// working directory and is exercised by the service-layer tests.
//
// Refresh moved from `r` (now Restack) to `ctrl+r` when local
// actions landed; iterating the keymap rather than hard-coding the
// string keeps this test honest if the binding moves again.
func TestUpdateRefreshDispatchesLoad(t *testing.T) {
	m := New(context.Background(), service.New(""))

	for _, key := range m.keys.Refresh.Keys {
		t.Run(key, func(t *testing.T) {
			_, cmd := m.Update(syntheticKey(key))
			if cmd == nil {
				t.Fatalf("Update on refresh key %q returned nil cmd, want a load command", key)
			}
			msg := cmd()
			if _, ok := msg.(snapshotMsg); !ok {
				t.Fatalf("cmd() = %T, want snapshotMsg", msg)
			}
		})
	}
}

// TestUpdateStoresSnapshot guarantees an arriving snapshotMsg lands
// on the model — otherwise the dashboard would be stuck on
// "loading…" forever even after a successful read.
func TestUpdateStoresSnapshot(t *testing.T) {
	m := New(context.Background(), nil)
	wantSnap := &service.DashboardSnapshot{Trunk: "main", Current: "feat-a"}
	next, _ := m.Update(snapshotMsg{snap: wantSnap})
	got := next.(Model)
	if got.snapshot != wantSnap {
		t.Fatalf("snapshot not stored after snapshotMsg")
	}
	if got.loadErr != nil {
		t.Fatalf("loadErr = %v, want nil after a successful load", got.loadErr)
	}
}

// TestUpdateStoresLoadError makes sure failures are surfaced rather
// than silently dropped. The dashboard renderer uses loadErr to
// switch from "loading…" to a red error banner.
func TestUpdateStoresLoadError(t *testing.T) {
	m := New(context.Background(), nil)
	wantErr := errors.New("simulated failure")
	next, _ := m.Update(snapshotMsg{err: wantErr})
	got := next.(Model)
	if got.loadErr != wantErr {
		t.Fatalf("loadErr = %v, want %v", got.loadErr, wantErr)
	}
}

// TestViewWhileQuittingIsBlank pins the alt-screen tear-down
// contract: View must return "" once quitting is set so bubbletea's
// alt-screen exit lands on a clean terminal.
func TestViewWhileQuittingIsBlank(t *testing.T) {
	m := New(context.Background(), nil)
	m.quitting = true
	if got := m.View(); got != "" {
		t.Fatalf("View while quitting = %q, want empty", got)
	}
}

// TestViewLoadingState verifies the placeholder screen renders
// before the first snapshot arrives. The dashboard footer must
// always be present so the user can see the quit hint and isn't
// stranded by an unresponsive blank screen.
func TestViewLoadingState(t *testing.T) {
	m := New(context.Background(), nil)
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Fatalf("View() missing loading placeholder. Got:\n%s", out)
	}
	if !strings.Contains(out, "quit") {
		t.Fatalf("View() missing quit hint in footer. Got:\n%s", out)
	}
}

// TestViewErrorBannerWhenLoadFailed pins the failure-mode UX: the
// red banner replaces the "loading…" placeholder and includes the
// error text so the user can act on it.
func TestViewErrorBannerWhenLoadFailed(t *testing.T) {
	m := New(context.Background(), nil)
	m.loadErr = errors.New("trunk not found")
	out := m.View()
	if !strings.Contains(out, "load failed") {
		t.Fatalf("View() missing failure banner. Got:\n%s", out)
	}
	if !strings.Contains(out, "trunk not found") {
		t.Fatalf("View() should surface the underlying error message. Got:\n%s", out)
	}
}

// TestViewRendersTreeAndPausedBanner exercises the wired snapshot
// → View pipeline end-to-end on a non-trivial stack: trunk plus
// two tracked branches, one of them currently checked out, and an
// active paused-restack banner. We check substrings rather than
// pixel-perfect output so trivial styling tweaks don't churn the
// test.
func TestViewRendersTreeAndPausedBanner(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.snapshot.Paused = &service.PausedSnapshot{Branch: "feat-a", Origin: "restack"}
	out := m.View()
	for _, want := range []string{"main", "feat-a", "feat-b", "← current", "paused on feat-a"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// syntheticKey builds a tea.KeyMsg whose String() matches name.
// Used so tests can drive named keys (ctrl+c, esc, …) without
// reaching into bubbletea's unexported key tables.
func syntheticKey(name string) tea.KeyMsg {
	switch name {
	case "ctrl+c":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC})
	case "ctrl+r":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlR})
	case "ctrl+d":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlD})
	case "ctrl+u":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlU})
	case "esc":
		return tea.KeyMsg(tea.Key{Type: tea.KeyEsc})
	case "enter":
		return tea.KeyMsg(tea.Key{Type: tea.KeyEnter})
	case "up":
		return tea.KeyMsg(tea.Key{Type: tea.KeyUp})
	case "down":
		return tea.KeyMsg(tea.Key{Type: tea.KeyDown})
	case "home":
		return tea.KeyMsg(tea.Key{Type: tea.KeyHome})
	case "end":
		return tea.KeyMsg(tea.Key{Type: tea.KeyEnd})
	case "pgup":
		return tea.KeyMsg(tea.Key{Type: tea.KeyPgUp})
	case "pgdown":
		return tea.KeyMsg(tea.Key{Type: tea.KeyPgDown})
	case "tab":
		return tea.KeyMsg(tea.Key{Type: tea.KeyTab})
	case "shift+tab":
		return tea.KeyMsg(tea.Key{Type: tea.KeyShiftTab})
	case "ctrl+p":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlP})
	case "ctrl+k":
		return tea.KeyMsg(tea.Key{Type: tea.KeyCtrlK})
	case "backspace":
		return tea.KeyMsg(tea.Key{Type: tea.KeyBackspace})
	case "space":
		return tea.KeyMsg(tea.Key{Type: tea.KeySpace})
	default:
		return tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(name)})
	}
}
