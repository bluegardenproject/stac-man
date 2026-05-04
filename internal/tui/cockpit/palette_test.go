package cockpit

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluegardenproject/stac-man/internal/service"
)

// withPaletteOpen returns a Model already in the palette screen
// against the canonical fixture, so tests can drive query/cursor
// keys without re-running the open path.
func withPaletteOpen(t *testing.T, branch string) Model {
	t.Helper()
	m := withActionSnapshot(t, branch)
	// Cache detail so the Diff palette item shows up too — tests
	// rely on a stable item set.
	m.details[branch] = &service.BranchView{Branch: branch}
	return m.openPalette()
}

// TestPaletteOpenFromDashboard pins the entry contract: ctrl+p (and
// its synonyms) opens the palette from the dashboard. Iterating
// the keymap rather than hard-coding the strings keeps this honest
// if a synonym is added later.
func TestPaletteOpenFromDashboard(t *testing.T) {
	for _, key := range DefaultKeymap().PaletteOpen.Keys {
		t.Run(key, func(t *testing.T) {
			m := withActionSnapshot(t, "feat-a")
			next, _ := m.Update(syntheticKey(key))
			got := next.(Model)
			if got.screen != screenPalette {
				t.Fatalf("screen = %v after %q, want screenPalette", got.screen, key)
			}
			if got.prevScreen != screenDashboard {
				t.Fatalf("prevScreen = %v, want screenDashboard so close returns home", got.prevScreen)
			}
			if got.paletteQuery != "" {
				t.Fatalf("paletteQuery = %q, want empty on open", got.paletteQuery)
			}
		})
	}
}

// TestPaletteEscClosesAndRestoresPrevScreen mirrors the entry test
// for the exit path. Esc must drop us back where we came from
// rather than always landing on the dashboard, otherwise the
// palette would feel destructive when opened from diff/conflict.
func TestPaletteEscClosesAndRestoresPrevScreen(t *testing.T) {
	m := New(context.Background(), nil)
	m.snapshot = sampleSnapshot()
	m.screen = screenDiff // simulate opening palette from diff
	m = m.openPalette()

	next, _ := m.Update(syntheticKey("esc"))
	got := next.(Model)
	if got.screen != screenDiff {
		t.Fatalf("screen = %v after esc, want screenDiff (the prevScreen)", got.screen)
	}
	if got.paletteQuery != "" {
		t.Fatalf("paletteQuery = %q after close, want cleared", got.paletteQuery)
	}
}

// TestPaletteQuitGatedWhenOpen documents the most important UX
// safety: typing `q` to filter "quit" must not actually quit. Only
// ctrl+c is honoured as the universal escape hatch.
func TestPaletteQuitGatedWhenOpen(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")

	next, cmd := m.Update(syntheticKey("q"))
	got := next.(Model)
	if got.quitting {
		t.Fatalf("typing q in palette quit the cockpit")
	}
	if cmd != nil {
		if _, ok := cmd().(tea.QuitMsg); ok {
			t.Fatalf("typing q dispatched a Quit cmd")
		}
	}
	if got.paletteQuery != "q" {
		t.Fatalf("paletteQuery = %q, want %q (q must append to query)", got.paletteQuery, "q")
	}
}

// TestPaletteCtrlCAlwaysQuits is the inverse of the above: even
// inside the palette, ctrl+c must exit. Trapping the user with no
// way out would be a worse failure mode than the global quit
// race.
func TestPaletteCtrlCAlwaysQuits(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	next, cmd := m.Update(syntheticKey("ctrl+c"))
	got := next.(Model)
	if !got.quitting {
		t.Fatalf("ctrl+c in palette did not set quitting")
	}
	if cmd == nil {
		t.Fatalf("ctrl+c in palette returned nil cmd, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c cmd() = %T, want tea.QuitMsg", cmd())
	}
}

// TestPaletteRunesAndBackspaceEditQuery covers basic text input.
// We type "rest", then backspace, then "ack" and assert the query
// reaches "restack".
func TestPaletteRunesAndBackspaceEditQuery(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	for _, r := range "rest" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}
	if m.paletteQuery != "rest" {
		t.Fatalf("paletteQuery = %q, want %q", m.paletteQuery, "rest")
	}
	next, _ := m.Update(syntheticKey("backspace"))
	m = next.(Model)
	if m.paletteQuery != "res" {
		t.Fatalf("paletteQuery after backspace = %q, want %q", m.paletteQuery, "res")
	}
	for _, r := range "tack" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}
	if m.paletteQuery != "restack" {
		t.Fatalf("final paletteQuery = %q, want %q", m.paletteQuery, "restack")
	}
}

// TestPaletteSpaceAppendedToQuery covers multi-token search:
// pressing space appends a literal space so the user can type
// "restack feat" to filter to a specific branch.
func TestPaletteSpaceAppendedToQuery(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	for _, r := range "restack" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}
	next, _ := m.Update(syntheticKey("space"))
	m = next.(Model)
	if m.paletteQuery != "restack " {
		t.Fatalf("paletteQuery = %q, want %q", m.paletteQuery, "restack ")
	}
}

// TestPaletteUpDownMovesCursor exercises the cursor matrix while a
// non-trivial filtered list is in play. Filter to "rest" so we
// reliably get >1 match (the cursor-targeted Restack item plus
// any HEAD-targeted items that don't match — the AND-of-tokens
// matcher narrows things to one or two items either way; we
// don't rely on a specific count, only that the cursor
// respects the bounds of whatever it is).
func TestPaletteUpDownMovesCursor(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	// No query — full item list, several items present.
	items := buildPaletteItems(m)
	if len(items) < 2 {
		t.Fatalf("need >=2 palette items for this test, got %d", len(items))
	}

	next, _ := m.Update(syntheticKey("down"))
	got := next.(Model)
	if got.paletteCursor != 1 {
		t.Fatalf("paletteCursor after Down = %d, want 1", got.paletteCursor)
	}

	next2, _ := got.Update(syntheticKey("up"))
	got2 := next2.(Model)
	if got2.paletteCursor != 0 {
		t.Fatalf("paletteCursor after Up = %d, want 0", got2.paletteCursor)
	}

	// Up at top is a no-op.
	next3, _ := got2.Update(syntheticKey("up"))
	got3 := next3.(Model)
	if got3.paletteCursor != 0 {
		t.Fatalf("paletteCursor after Up at top = %d, want 0 (clamped)", got3.paletteCursor)
	}
}

// TestPaletteEnterRunsItem pins the dispatch contract: pressing
// Enter on the cursor item invokes its Run closure and leaves the
// palette. The chosen item here is Refresh (always at index 0),
// which produces a snapshotMsg via loadSnapshotCmd — easy to
// assert on without depending on git state.
func TestPaletteEnterRunsItem(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	// Type "reload" so only the Refresh item matches (its label is
	// "reload" per DefaultKeymap). This narrows the dispatch
	// target unambiguously.
	for _, r := range "reload" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}

	next, cmd := m.Update(syntheticKey("enter"))
	got := next.(Model)
	if got.screen == screenPalette {
		t.Fatalf("screen still palette after Enter; expected close to dashboard")
	}
	if cmd == nil {
		t.Fatalf("Enter on Refresh item returned nil cmd, want a snapshot reload")
	}
	if _, ok := cmd().(snapshotMsg); !ok {
		t.Fatalf("cmd() = %T, want snapshotMsg from Refresh dispatch", cmd())
	}
}

// TestPaletteEnterOnEmptyResultsIsNoOp prevents an off-by-one
// crash: when the query filters everything out, Enter must do
// nothing rather than panic on items[0].
func TestPaletteEnterOnEmptyResultsIsNoOp(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	// Use a token that won't match any label.
	for _, r := range "zzznever" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}

	next, cmd := m.Update(syntheticKey("enter"))
	got := next.(Model)
	if got.screen != screenPalette {
		t.Fatalf("Enter on empty results closed palette: screen=%v", got.screen)
	}
	if cmd != nil {
		t.Fatalf("Enter on empty results dispatched %T, want nil", cmd())
	}
}

// TestBuildPaletteItemsTrunkGuard covers the action-list
// invariant: when the cursor is on trunk, Restack/Track/Untrack
// MUST be hidden — same guard the keypress dispatcher applies.
// Otherwise the palette would offer items that no-op at the
// service layer.
func TestBuildPaletteItemsTrunkGuard(t *testing.T) {
	m := withPaletteOpen(t, "main")
	items := buildPaletteItems(m)

	for _, label := range []string{"restack", "track", "untrack"} {
		for _, it := range items {
			if strings.HasPrefix(strings.ToLower(it.Label), label+" ") {
				t.Fatalf("palette includes %q while cursor on trunk: %q", label, it.Label)
			}
		}
	}
}

// TestBuildPaletteItemsIncludesDiffOnlyWhenCached pins the second
// entry guard: Diff requires the BranchView to be cached. Without
// this gate, palette would present a Diff entry that opens an
// empty viewer.
func TestBuildPaletteItemsIncludesDiffOnlyWhenCached(t *testing.T) {
	m := withActionSnapshot(t, "feat-a")
	// No detail cached.
	m = m.openPalette()
	items := buildPaletteItems(m)
	for _, it := range items {
		if strings.HasPrefix(it.Label, "diff ") {
			t.Fatalf("palette offered Diff entry without cached detail: %q", it.Label)
		}
	}

	// Now cache detail and re-build.
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}
	items = buildPaletteItems(m)
	found := false
	for _, it := range items {
		if strings.HasPrefix(it.Label, "diff feat-a") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("palette missing Diff entry once detail cached. Got items:\n%+v", items)
	}
}

// TestFilterPaletteItemsAndOfTokens is a focused unit on the
// matcher, separate from the Update pipeline. Tokens are AND-ed
// (every token must appear) and matching is case-insensitive.
func TestFilterPaletteItemsAndOfTokens(t *testing.T) {
	items := []paletteAction{
		{Label: "restack feat-a"},
		{Label: "checkout feat-a"},
		{Label: "submit"},
		{Label: "absorb"},
	}

	cases := []struct {
		query string
		want  []string
	}{
		{"", []string{"restack feat-a", "checkout feat-a", "submit", "absorb"}},
		{"feat", []string{"restack feat-a", "checkout feat-a"}},
		{"FEAT", []string{"restack feat-a", "checkout feat-a"}}, // case-insensitive
		{"restack feat", []string{"restack feat-a"}},            // multi-token AND
		{"abs", []string{"absorb"}},
		{"   ", []string{"restack feat-a", "checkout feat-a", "submit", "absorb"}}, // whitespace = empty
		{"zzz", nil},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			got := filterPaletteItems(items, tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("filter %q: got %d items, want %d (%+v vs %v)", tc.query, len(got), len(tc.want), got, tc.want)
			}
			for i, it := range got {
				if it.Label != tc.want[i] {
					t.Fatalf("filter %q [%d]: got %q, want %q", tc.query, i, it.Label, tc.want[i])
				}
			}
		})
	}
}

// TestViewPaletteRendersQueryAndItems is the View smoke test:
// query echoes back, the filtered items render, the footer hint
// is present.
func TestViewPaletteRendersQueryAndItems(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	m.paletteQuery = "rest"

	out := m.View()
	for _, want := range []string{
		"command palette",
		"rest",
		"restack feat-a",
		"esc close",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("View() missing %q. Got:\n%s", want, out)
		}
	}
}

// TestViewPaletteEmptyResultsHints pins the no-match UX so a
// typo doesn't render as an empty pane.
func TestViewPaletteEmptyResultsHints(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	m.paletteQuery = "zzznever"

	out := m.View()
	if !strings.Contains(out, "no matching actions") {
		t.Fatalf("View() missing no-match hint. Got:\n%s", out)
	}
}

// TestPaletteOpenIdempotentWhilePaletteShown documents that
// pressing ctrl+p while the palette is already open does not
// re-open (which would clobber the running query). Same gate as
// the Update guard.
func TestPaletteOpenIdempotentWhilePaletteShown(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	for _, r := range "rest" {
		next, _ := m.Update(syntheticKey(string(r)))
		m = next.(Model)
	}
	next, _ := m.Update(syntheticKey("ctrl+p"))
	got := next.(Model)
	if got.paletteQuery != "rest" {
		t.Fatalf("paletteQuery = %q after re-open press, want %q (must not reset mid-typing)", got.paletteQuery, "rest")
	}
}

// TestRefreshKeyDoesNotFireWhilePaletteOpen is the gating mirror
// for refresh: typing `R` to filter "Restack" must not flush the
// caches.
func TestRefreshKeyDoesNotFireWhilePaletteOpen(t *testing.T) {
	m := withPaletteOpen(t, "feat-a")
	m.details["feat-a"] = &service.BranchView{Branch: "feat-a"}
	m.lastAction = &actionResult{Verb: "restack", Branch: "feat-a"}

	next, cmd := m.Update(syntheticKey("R"))
	got := next.(Model)
	if cmd != nil {
		if _, ok := cmd().(snapshotMsg); ok {
			t.Fatalf("R in palette dispatched a snapshot reload")
		}
	}
	if len(got.details) == 0 {
		t.Fatalf("R in palette cleared the detail cache")
	}
	if got.lastAction == nil {
		t.Fatalf("R in palette cleared lastAction")
	}
	if got.paletteQuery != "R" {
		t.Fatalf("paletteQuery = %q, want %q", got.paletteQuery, "R")
	}
}
