package cockpit

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestHelpOpenFromAnyScreen pins the entry contract: pressing ?
// from the dashboard, conflict resolver, and diff viewer all open
// the help overlay and remember the prevScreen so close returns
// to the same place. Per-screen entry tests catch regressions in
// the global-key gate.
func TestHelpOpenFromAnyScreen(t *testing.T) {
	cases := []struct {
		name string
		from screen
	}{
		{"from dashboard", screenDashboard},
		{"from conflict", screenConflict},
		{"from diff", screenDiff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(context.Background(), nil)
			m.snapshot = sampleSnapshot()
			m.screen = tc.from

			next, _ := m.Update(syntheticKey("?"))
			got := next.(Model)
			if got.screen != screenHelp {
				t.Fatalf("screen = %v after ?, want screenHelp", got.screen)
			}
			if got.prevScreen != tc.from {
				t.Fatalf("prevScreen = %v, want %v", got.prevScreen, tc.from)
			}
		})
	}
}

// TestHelpDoesNotOpenFromPalette mirrors the gating guarantees we
// already have for Quit/Refresh: ? in the palette appends to the
// query, it does NOT open another overlay on top of one.
func TestHelpDoesNotOpenFromPalette(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	next, _ := m.Update(syntheticKey("?"))
	got := next.(Model)
	if got.screen != screenPalette {
		t.Fatalf("? in palette switched screen to %v, want screenPalette", got.screen)
	}
	if got.paletteQuery != "?" {
		t.Fatalf("paletteQuery = %q, want %q (? must be a queryable rune in palette)", got.paletteQuery, "?")
	}
}

// TestHelpToggleClosesOnSecondPress documents the muscle-memory
// contract: pressing ? while help is up closes it. Without this
// users would have to learn that esc is the only close key.
func TestHelpToggleClosesOnSecondPress(t *testing.T) {
	m := New(context.Background(), nil)
	m.screen = screenDashboard
	m = m.openHelp()

	next, _ := m.Update(syntheticKey("?"))
	got := next.(Model)
	if got.screen != screenDashboard {
		t.Fatalf("second ? did not close help: screen = %v", got.screen)
	}
}

// TestHelpEscClosesAndRestoresPrevScreen mirrors the toggle test
// for the more conventional close gesture and validates that the
// prevScreen pop works the same way as the palette's.
func TestHelpEscClosesAndRestoresPrevScreen(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.screen = screenDiff
	m = m.openHelp()

	next, _ := m.Update(syntheticKey("esc"))
	got := next.(Model)
	if got.screen != screenDiff {
		t.Fatalf("esc returned to %v, want screenDiff", got.screen)
	}
}

// TestBuildHelpSectionsCoversAllBindings is the anti-drift guard
// the whole help overlay exists for. Every KeyBinding field on
// Keymap must appear in exactly one section, otherwise users
// would have a binding that's reachable but undiscoverable.
//
// Implementation: reflect over Keymap, collect every KeyBinding
// field's Help text, then assert each one shows up in the section
// list. We use Help (not Keys) as the identity key because a
// KeyBinding with the same Help in two sections is itself a bug.
func TestBuildHelpSectionsCoversAllBindings(t *testing.T) {
	k := DefaultKeymap()
	v := reflect.ValueOf(k)
	t.Logf("checking %d Keymap fields", v.NumField())

	wantHelps := map[string]bool{}
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Type() != reflect.TypeOf(KeyBinding{}) {
			continue
		}
		bd := field.Interface().(KeyBinding)
		if bd.Help == "" {
			t.Fatalf("Keymap.%s has empty Help — every binding must have a label", v.Type().Field(i).Name)
		}
		wantHelps[bd.Help] = false
	}

	sections := buildHelpSections(k)
	for _, s := range sections {
		for _, bd := range s.Bindings {
			if _, ok := wantHelps[bd.Help]; !ok {
				t.Errorf("section %q has binding %q not in Keymap (typo? or stale?)", s.Title, bd.Help)
				continue
			}
			if wantHelps[bd.Help] {
				t.Errorf("binding %q appears in multiple sections — must be exactly one", bd.Help)
			}
			wantHelps[bd.Help] = true
		}
	}

	for help, seen := range wantHelps {
		if !seen {
			t.Errorf("binding %q not in any help section — discoverability gap", help)
		}
	}
}

// TestViewHelpRendersEverySectionTitle pins the layout contract:
// every section that buildHelpSections returns must produce a
// visible header in the rendered view. Catches the bug where
// renderHelpSections accidentally drops a section.
func TestViewHelpRendersEverySectionTitle(t *testing.T) {
	m := New(context.Background(), nil)
	m.screen = screenHelp

	out := m.View()
	for _, s := range buildHelpSections(m.keys) {
		if !strings.Contains(out, s.Title) {
			t.Fatalf("View() missing section title %q. Got:\n%s", s.Title, out)
		}
	}
}

// TestViewHelpRendersEveryBindingHelp is the "no binding hidden"
// View-level mirror of TestBuildHelpSectionsCoversAllBindings:
// every Keymap field's Help text must appear in the rendered
// output. We accept either the bare Help text or any of its key
// strings as the assertion target so styling tweaks don't churn
// the test.
func TestViewHelpRendersEveryBindingHelp(t *testing.T) {
	m := New(context.Background(), nil)
	m.screen = screenHelp
	out := m.View()

	v := reflect.ValueOf(m.keys)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Type() != reflect.TypeOf(KeyBinding{}) {
			continue
		}
		bd := field.Interface().(KeyBinding)
		if !strings.Contains(out, bd.Help) {
			t.Fatalf("View() missing binding label %q (Keymap.%s). Got:\n%s",
				bd.Help, v.Type().Field(i).Name, out)
		}
	}
}

// TestViewHelpRendersCloseHint guards the affordance: the user
// must be told how to dismiss the overlay or they'll hammer keys
// trying to escape.
func TestViewHelpRendersCloseHint(t *testing.T) {
	m := New(context.Background(), nil)
	m.screen = screenHelp
	out := m.View()
	for _, want := range []string{"esc", "close"} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q in close hint. Got:\n%s", want, out)
		}
	}
}

// TestSplitSectionsAlternatesColumns documents the column-split
// rule. Even-indexed sections go left, odd-indexed go right.
// Pulled out as a unit so balancing changes don't have to be
// re-derived from rendered output.
func TestSplitSectionsAlternatesColumns(t *testing.T) {
	sections := []helpSection{
		{Title: "a"}, {Title: "b"}, {Title: "c"},
		{Title: "d"}, {Title: "e"},
	}
	left, right := splitSections(sections)
	wantLeft := []string{"a", "c", "e"}
	wantRight := []string{"b", "d"}

	if len(left) != len(wantLeft) || len(right) != len(wantRight) {
		t.Fatalf("len(left)=%d len(right)=%d, want %d/%d", len(left), len(right), len(wantLeft), len(wantRight))
	}
	for i, w := range wantLeft {
		if left[i].Title != w {
			t.Fatalf("left[%d] = %q, want %q", i, left[i].Title, w)
		}
	}
	for i, w := range wantRight {
		if right[i].Title != w {
			t.Fatalf("right[%d] = %q, want %q", i, right[i].Title, w)
		}
	}
}

// TestHelpDoesNotDispatchActionsOnReturn covers the "no side
// effects" contract: closing help with esc must not fire any
// stale snapshotMsg or actionResult cmd. The overlay is read-only
// and the close path is pure state transition.
func TestHelpDoesNotDispatchActionsOnReturn(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.screen = screenHelp

	_, cmd := m.Update(syntheticKey("esc"))
	if cmd != nil {
		t.Fatalf("closing help dispatched %T, want nil cmd", cmd())
	}
}
