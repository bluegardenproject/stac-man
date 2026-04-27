package tui

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
)

// PickerResult is what [PickBranch] returns once the user acts on the
// picker. Branch is the chosen branch when Cancelled is false; the
// caller is responsible for actually checking it out.
type PickerResult struct {
	Branch    string
	Cancelled bool
}

// PickBranch shows an interactive Bubble Tea picker over the supplied
// CheckoutItem tree. The cursor starts on the row matching current
// (typically the branch HEAD is on) so a confused user can press Enter
// to no-op, or Esc/q/Ctrl+C to bail without changing branches.
//
// Returns Cancelled=true when the user dismisses the picker. If
// items is empty the caller should fall back to a flat listing —
// PickBranch returns Cancelled=true in that case rather than show an
// empty UI.
func PickBranch(title string, current string, items []service.CheckoutItem) (PickerResult, error) {
	if len(items) == 0 {
		return PickerResult{Cancelled: true}, nil
	}

	cursor := 0
	for i, it := range items {
		if it.Branch == current && it.IsCurrent {
			cursor = i
			break
		}
	}

	m := pickerModel{
		title:  title,
		items:  items,
		cursor: cursor,
	}

	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return PickerResult{}, fmt.Errorf("picker: %w", err)
	}
	fm, ok := final.(pickerModel)
	if !ok {
		return PickerResult{}, fmt.Errorf("picker: unexpected model %T", final)
	}
	if fm.cancelled || !fm.chosen {
		return PickerResult{Cancelled: true}, nil
	}
	return PickerResult{Branch: fm.items[fm.cursor].Branch}, nil
}

// pickerModel is the Bubble Tea model backing PickBranch. It keeps
// just a cursor and exit flags — all data is precomputed by the
// service so the model stays trivially testable.
type pickerModel struct {
	title     string
	items     []service.CheckoutItem
	cursor    int
	chosen    bool
	cancelled bool
}

func (m pickerModel) Init() tea.Cmd { return nil }

func (m pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "ctrl+c", "esc", "q":
		m.cancelled = true
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.items) - 1
	case "enter":
		m.chosen = true
		return m, tea.Quit
	}
	return m, nil
}

func (m pickerModel) View() string {
	// Returning an empty string on exit erases the picker so the
	// command's own "checked out X" toast lands on a clean line.
	if m.chosen || m.cancelled {
		return ""
	}

	var b strings.Builder
	if m.title != "" {
		b.WriteString(ui.Render(theme.Header, m.title) + "\n\n")
	}
	for i, it := range m.items {
		b.WriteString(renderRow(it, i == m.cursor))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(ui.Render(theme.Dimmed, "↑/↓ move  •  enter checkout  •  esc cancel"))
	return b.String()
}

// renderRow builds one tree-shaped line: cursor caret, ancestor
// pipes, connector, branch name, status suffix.
func renderRow(it service.CheckoutItem, selected bool) string {
	caret := "  "
	if selected {
		caret = ui.Render(theme.Accent, "▸ ")
	}

	var prefix strings.Builder
	for _, ancestorIsLast := range it.AncestorIsLast {
		if ancestorIsLast {
			prefix.WriteString("   ")
		} else {
			prefix.WriteString("│  ")
		}
	}
	if !it.IsTrunk {
		if it.IsLastChild {
			prefix.WriteString("└─ ")
		} else {
			prefix.WriteString("├─ ")
		}
	}

	nameStyle := branchStyle(it)
	name := it.Branch
	suffix := ""
	switch {
	case it.IsCurrent:
		suffix = "  ← current"
	case it.NeedsRestack:
		suffix = "  (needs restack)"
	}

	row := caret + ui.Render(theme.Dimmed, prefix.String()) + ui.Render(nameStyle, name+suffix)
	if it.PR > 0 {
		row += ui.Render(theme.Dimmed, fmt.Sprintf("  #%d", it.PR))
	}

	if selected {
		// A subtle bold pass over an already-styled row keeps the
		// foreground colours but emphasises the cursor's target.
		row = lipgloss.NewStyle().Bold(true).Render(row)
	}
	return row
}

func branchStyle(it service.CheckoutItem) lipgloss.Style {
	switch {
	case it.IsTrunk:
		return theme.BranchTrunk
	case it.IsCurrent:
		return theme.BranchCurrent
	case it.NeedsRestack:
		return theme.BranchNeedsRestack
	default:
		return theme.BranchHealthy
	}
}

// RenderTree returns the same list PickBranch shows, but as a static
// string — used as the non-TTY fallback so `sm checkout | cat` still
// produces useful output. The cursor caret is replaced with a current-
// branch marker to keep parity with the interactive view.
func RenderTree(w io.Writer, title string, items []service.CheckoutItem) error {
	if title != "" {
		if _, err := fmt.Fprintln(w, ui.Render(theme.Header, title)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	for _, it := range items {
		marker := "  "
		if it.IsCurrent {
			marker = ui.Render(theme.OK, "→ ")
		}

		var prefix strings.Builder
		for _, ancestorIsLast := range it.AncestorIsLast {
			if ancestorIsLast {
				prefix.WriteString("   ")
			} else {
				prefix.WriteString("│  ")
			}
		}
		if !it.IsTrunk {
			if it.IsLastChild {
				prefix.WriteString("└─ ")
			} else {
				prefix.WriteString("├─ ")
			}
		}

		name := it.Branch
		suffix := ""
		switch {
		case it.IsCurrent:
			suffix = "  ← current"
		case it.NeedsRestack:
			suffix = "  (needs restack)"
		}
		line := marker + ui.Render(theme.Dimmed, prefix.String()) + ui.Render(branchStyle(it), name+suffix)
		if it.PR > 0 {
			line += ui.Render(theme.Dimmed, fmt.Sprintf("  #%d", it.PR))
		}
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return nil
}
