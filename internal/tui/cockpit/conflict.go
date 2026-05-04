package cockpit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
)

// updateConflict handles keys while the conflict resolver is the
// active screen. Cursor movement is limited to the conflict-paths
// list; everything else is one-shot dispatch.
//
// We deliberately don't gate on m.snapshot.Paused == nil here:
// reaching a key with no paused state is a routing bug, but the
// safest response is to silently route back rather than panic.
func (m Model) updateConflict(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.snapshot == nil || m.snapshot.Paused == nil {
		// Nothing to act on; let Back take the user out and any
		// other key falls through.
		if m.keys.Back.Matches(msg) {
			m.screen = screenDashboard
			return m, nil
		}
		return m, nil
	}

	paths := m.snapshot.Paused.ConflictPaths
	switch {
	case m.keys.Up.Matches(msg):
		if m.conflictCursor > 0 {
			m.conflictCursor--
		}
		return m, nil
	case m.keys.Down.Matches(msg):
		if m.conflictCursor < len(paths)-1 {
			m.conflictCursor++
		}
		return m, nil
	case m.keys.Top.Matches(msg):
		m.conflictCursor = 0
		return m, nil
	case m.keys.Bottom.Matches(msg):
		if n := len(paths); n > 0 {
			m.conflictCursor = n - 1
		}
		return m, nil

	case m.keys.ConflictContinue.Matches(msg):
		return m, m.conflictContinueCmd()
	case m.keys.ConflictAbort.Matches(msg):
		return m, m.conflictAbortCmd()
	case m.keys.ConflictEdit.Matches(msg):
		if cmd := m.conflictEditCmd(paths); cmd != nil {
			return m, cmd
		}
		return m, nil

	case m.keys.Back.Matches(msg):
		// Manual back: let the user browse the dashboard while a
		// rebase is paused. The paused banner stays on the
		// dashboard so they can re-enter via the snapshot's
		// firstLoad-into-paused routing on the next refresh.
		m.screen = screenDashboard
		return m, nil
	}
	return m, nil
}

// conflictContinueCmd dispatches a RestackContinue. Verb is taken
// from the binding's help label so the status line and help overlay
// stay aligned.
func (m Model) conflictContinueCmd() tea.Cmd {
	return runActionCmd(m.keys.ConflictContinue.Help, "", func() error {
		return m.svc.RestackContinue(m.ctx)
	})
}

// conflictAbortCmd dispatches a RestackAbort. Verb mirrors the
// binding label, same as continue.
func (m Model) conflictAbortCmd() tea.Cmd {
	return runActionCmd(m.keys.ConflictAbort.Help, "", func() error {
		return m.svc.RestackAbort(m.ctx)
	})
}

// editorCommand resolves the user's editor preference to an
// exec.Cmd opening path. $VISUAL wins over $EDITOR (POSIX
// convention: VISUAL is the rich editor, EDITOR the line-mode
// fallback) and we fall back to vi so a sane default ships.
//
// Pulled out of conflictEditCmd so tests can pin the resolution
// rules without spawning real processes.
func editorCommand(path string) *exec.Cmd {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	// Honour shell-style editor settings like `code -w` or
	// `emacsclient -nw` by splitting on whitespace. This matches
	// what git does for $GIT_EDITOR and is the least-surprise
	// behaviour for users with multi-token editor configs.
	parts := strings.Fields(editor)
	args := append(parts[1:], path)
	return exec.Command(parts[0], args...)
}

// editorFinishedMsg is delivered after tea.ExecProcess returns
// from the editor. We don't roll the result into actionResult
// because editor exits aren't service-layer outcomes — there's no
// snapshot to refresh and no conflict resolution semantics to
// model. The status line still surfaces the exit error so a
// "EDITOR not found" error is visible.
type editorFinishedMsg struct {
	path string
	err  error
}

// conflictEditCmd shells out to $EDITOR for the conflict path under
// the cursor. Returns nil when there are no paths to edit so the
// keypress is a silent no-op rather than spawning a blank editor
// session.
//
// We pass the editor's stdio through so it gets a real terminal —
// tea.ExecProcess releases the alt-screen first, runs the command
// against the user's actual TTY, then reattaches.
func (m Model) conflictEditCmd(paths []string) tea.Cmd {
	if len(paths) == 0 {
		return nil
	}
	idx := m.conflictCursor
	if idx < 0 || idx >= len(paths) {
		idx = 0
	}
	path := paths[idx]
	c := editorCommand(path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{path: path, err: err}
	})
}

// viewConflict renders the conflict resolver. Three sections,
// always in the same order so the layout doesn't jump as the
// cursor moves:
//
//  1. Header — paused branch + origin verb
//  2. Pending queue — what's still left to rebase after this
//     branch resolves
//  3. Conflict paths — cursor-driven list of unmerged files
//
// Returns a fallback view when entered without paused state so a
// routing bug doesn't paint a blank screen.
func (m Model) viewConflict() string {
	var b strings.Builder
	b.WriteString(ui.Banner("stac-man — conflict resolver") + "\n\n")

	if m.snapshot == nil || m.snapshot.Paused == nil {
		b.WriteString(ui.Render(theme.Dimmed, "no paused rebase — press esc to return to the dashboard\n"))
		b.WriteString("\n")
		b.WriteString(viewConflictFooter(m))
		return b.String()
	}

	p := m.snapshot.Paused
	b.WriteString(viewConflictHeader(p) + "\n\n")
	b.WriteString(viewConflictPending(p) + "\n\n")
	b.WriteString(viewConflictFiles(p, m.conflictCursor) + "\n")

	if status := viewActionStatus(m.lastAction); status != "" {
		b.WriteString("\n" + status + "\n")
	}

	b.WriteString("\n")
	b.WriteString(viewConflictFooter(m))
	return b.String()
}

// viewConflictHeader names the paused branch and the operation that
// paused (restack, modify-driven restack, …) so the user sees both
// "where" and "why" without scanning the dashboard.
func viewConflictHeader(p *service.PausedSnapshot) string {
	branch := p.Branch
	if branch == "" {
		branch = "(unknown)"
	}
	origin := p.Origin
	if origin == "" {
		origin = "rebase"
	}
	return ui.Render(theme.ErrorToast, fmt.Sprintf(" paused on %s ", branch)) +
		"  " +
		ui.Render(theme.Dimmed, "via "+origin)
}

// viewConflictPending renders the queue of branches still to be
// rebased after the current one resolves. When the queue is empty
// (the paused branch is the last one) we say so explicitly rather
// than render an empty section, which would look like a layout bug.
func viewConflictPending(p *service.PausedSnapshot) string {
	if len(p.Pending) == 0 {
		return ui.Render(theme.KeyLabel, "queue: ") + ui.Render(theme.Dimmed, "(none — this is the last branch)")
	}
	return ui.Render(theme.KeyLabel, "queue: ") + strings.Join(p.Pending, " → ")
}

// viewConflictFiles renders the cursor-driven list of unmerged
// files. The selected row gets the same caret + bold treatment as
// the dashboard tree so the cockpit feels visually consistent
// across screens.
func viewConflictFiles(p *service.PausedSnapshot, cursor int) string {
	var b strings.Builder
	count := len(p.ConflictPaths)
	b.WriteString(ui.Render(theme.KeyLabel, fmt.Sprintf("conflicts (%d):", count)) + "\n")
	if count == 0 {
		b.WriteString("  " + ui.Render(theme.Dimmed, "(no unmerged files — staged resolution detected, press c to continue)"))
		return b.String()
	}
	for i, path := range p.ConflictPaths {
		caret := "  "
		if i == cursor {
			caret = ui.Render(theme.Accent, "▸ ")
		}
		row := caret + ui.Render(theme.BranchNeedsRestack, path)
		if i == cursor {
			row = ui.Render(theme.Bold, row)
		}
		b.WriteString(row + "\n")
	}
	return b.String()
}

// viewConflictFooter renders the conflict screen's key-hint strip.
// Like the dashboard footer, this reads from m.keys so a binding
// rename anywhere in keymap.go propagates here automatically.
func viewConflictFooter(m Model) string {
	hint := func(b KeyBinding) string {
		key := strings.Join(b.Keys, "/")
		return ui.Render(theme.KeyHint, key) + " " + ui.Render(theme.KeyLabel, b.Help)
	}
	parts := []string{
		hint(m.keys.ConflictContinue),
		hint(m.keys.ConflictAbort),
		hint(m.keys.ConflictEdit),
		hint(m.keys.Up),
		hint(m.keys.Down),
		hint(m.keys.Back),
		hint(m.keys.Quit),
	}
	return strings.Join(parts, "  •  ")
}
