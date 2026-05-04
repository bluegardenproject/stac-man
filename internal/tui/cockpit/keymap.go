package cockpit

import tea "github.com/charmbracelet/bubbletea"

// KeyBinding pairs the tea.KeyMsg.String() values that should
// trigger an action with a short help label rendered by the help
// overlay (todo 11) and the command palette (todo 10). Centralising
// this here means Update's switch and the help/palette views all
// read from one source — they cannot drift.
//
// We deliberately don't pull in github.com/charmbracelet/bubbles/key
// for this; the existing internal/tui/picker.go matches keys by
// their string form too, and the lighter-weight surface keeps the
// dependency footprint flat.
type KeyBinding struct {
	// Keys is the list of key.String() values that match this
	// binding. Multiple entries make a binding accept synonyms
	// (e.g. arrow keys + vim keys for navigation).
	Keys []string
	// Help is the short label shown next to the keys in the help
	// overlay and command palette. Keep it imperative and < 30 chars.
	Help string
}

// Matches reports whether msg should trigger this binding.
func (b KeyBinding) Matches(msg tea.KeyMsg) bool {
	s := msg.String()
	for _, k := range b.Keys {
		if k == s {
			return true
		}
	}
	return false
}

// Keymap is the catalogue of every cockpit binding. New screens add
// fields here rather than hard-coding key strings inline so the help
// overlay automatically discovers them.
type Keymap struct {
	// Navigation — used by every screen that has a cursor.
	Up     KeyBinding
	Down   KeyBinding
	Top    KeyBinding
	Bottom KeyBinding

	// Dashboard data
	Refresh KeyBinding

	// Local actions — see internal/tui/cockpit/actions.go for the
	// dispatch logic. Actions split into two groups by semantics:
	//
	//   Cursor-targeted: Checkout, Restack, Track, Untrack — the
	//     selected branch is the operand. Trunk row is rejected
	//     for the mutating ones.
	//
	//   HEAD-targeted: Modify, Fold, Absorb, Undo — operate on the
	//     branch git is currently on, regardless of the cursor.
	//     Mirrors CLI semantics where these subcommands take no
	//     branch argument.
	Checkout KeyBinding
	Restack  KeyBinding
	Track    KeyBinding
	Untrack  KeyBinding
	Modify   KeyBinding
	Fold     KeyBinding
	Absorb   KeyBinding
	Undo     KeyBinding

	// Network actions — these shell out to the existing `sm`
	// subcommands via tea.ExecProcess so `gh` retains its real TTY
	// for prompts (auth flows, confirmations) and streaming output.
	// Running them in-process would either swallow `gh`'s prompts
	// or force the cockpit to re-implement every CLI render path.
	Submit KeyBinding
	Sync   KeyBinding
	Land   KeyBinding

	// Conflict resolver — only meaningful on the conflict screen.
	// Continue/Abort wrap the rebase --continue/--abort flow;
	// EditFile shells out to $EDITOR via tea.ExecProcess on the
	// path under the conflict cursor.
	ConflictContinue KeyBinding
	ConflictAbort    KeyBinding
	ConflictEdit     KeyBinding

	// Diff viewer — entered via Diff from the dashboard. Up/Down
	// reuse the cursor bindings to scroll the diff content one
	// line at a time; PageDown/PageUp scroll a page; NextCommit /
	// PrevCommit step through the branch's commit list. The split
	// keeps "navigate the diff itself" the primary action and
	// commit-stepping a deliberate gesture.
	Diff           KeyBinding
	DiffPageDown   KeyBinding
	DiffPageUp     KeyBinding
	DiffNextCommit KeyBinding
	DiffPrevCommit KeyBinding

	// Command palette — open / select / cancel. Only Open is a
	// dashboard binding; Select and Cancel are scoped to the
	// palette screen but live in the keymap so the help overlay
	// finds them via the same iteration as everything else.
	PaletteOpen   KeyBinding
	PaletteSelect KeyBinding

	// Global
	Back KeyBinding
	Help KeyBinding
	Quit KeyBinding
}

// DefaultKeymap returns the bindings the cockpit ships with. Tests
// construct Models from this so they don't have to re-declare the
// table.
//
// Refresh is bound to ctrl+r (with R as a synonym) rather than r so
// the lowercase r can mean Restack — the action map's documented
// binding from the cockpit plan.
func DefaultKeymap() Keymap {
	return Keymap{
		Up:     KeyBinding{Keys: []string{"up", "k"}, Help: "move up"},
		Down:   KeyBinding{Keys: []string{"down", "j"}, Help: "move down"},
		Top:    KeyBinding{Keys: []string{"home", "g"}, Help: "jump to top"},
		Bottom: KeyBinding{Keys: []string{"end", "G"}, Help: "jump to bottom"},

		Refresh: KeyBinding{Keys: []string{"ctrl+r", "R"}, Help: "reload"},

		Checkout: KeyBinding{Keys: []string{"enter"}, Help: "checkout"},
		Restack:  KeyBinding{Keys: []string{"r"}, Help: "restack"},
		Track:    KeyBinding{Keys: []string{"t"}, Help: "track"},
		Untrack:  KeyBinding{Keys: []string{"T"}, Help: "untrack"},
		Modify:   KeyBinding{Keys: []string{"m"}, Help: "amend"},
		Fold:     KeyBinding{Keys: []string{"f"}, Help: "fold"},
		Absorb:   KeyBinding{Keys: []string{"a"}, Help: "absorb"},
		Undo:     KeyBinding{Keys: []string{"u"}, Help: "undo"},

		Submit: KeyBinding{Keys: []string{"s"}, Help: "submit"},
		Sync:   KeyBinding{Keys: []string{"S"}, Help: "sync"},
		Land:   KeyBinding{Keys: []string{"L"}, Help: "land"},

		ConflictContinue: KeyBinding{Keys: []string{"c"}, Help: "continue rebase"},
		ConflictAbort:    KeyBinding{Keys: []string{"A"}, Help: "abort rebase"},
		ConflictEdit:     KeyBinding{Keys: []string{"e"}, Help: "edit file"},

		Diff:           KeyBinding{Keys: []string{"d"}, Help: "diff"},
		DiffPageDown:   KeyBinding{Keys: []string{"pgdown", "ctrl+d"}, Help: "page down"},
		DiffPageUp:     KeyBinding{Keys: []string{"pgup", "ctrl+u"}, Help: "page up"},
		DiffNextCommit: KeyBinding{Keys: []string{"tab", "n"}, Help: "next commit"},
		DiffPrevCommit: KeyBinding{Keys: []string{"shift+tab", "p"}, Help: "prev commit"},

		PaletteOpen:   KeyBinding{Keys: []string{"ctrl+p", "ctrl+k"}, Help: "command palette"},
		PaletteSelect: KeyBinding{Keys: []string{"enter"}, Help: "run"},

		Back: KeyBinding{Keys: []string{"esc"}, Help: "back"},
		Help: KeyBinding{Keys: []string{"?"}, Help: "show help"},
		Quit: KeyBinding{Keys: []string{"q", "ctrl+c"}, Help: "quit"},
	}
}
