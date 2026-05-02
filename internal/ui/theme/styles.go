package theme

import "github.com/charmbracelet/lipgloss"

// Pre-built styles. Render code should compose these rather than
// re-creating them inline so a palette tweak propagates everywhere.
var (
	// Generic emphasis
	Bold   = lipgloss.NewStyle().Bold(true)
	Dimmed = lipgloss.NewStyle().Foreground(Dim)
	Accent = lipgloss.NewStyle().Foreground(NeonMagenta).Bold(true)
	OK     = lipgloss.NewStyle().Foreground(NeonLime).Bold(true)
	Warn   = lipgloss.NewStyle().Foreground(NeonYellow)
	Fail   = lipgloss.NewStyle().Foreground(HotPink).Bold(true)
	Info   = lipgloss.NewStyle().Foreground(NeonCyan)

	// Banners / headers
	Title = lipgloss.NewStyle().
		Foreground(NeonPink).
		Bold(true)

	Subtitle = lipgloss.NewStyle().
			Foreground(NeonMagenta)

	Header = lipgloss.NewStyle().
		Foreground(NeonCyan).
		Bold(true).
		Underline(true)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(NeonCyan).
		Padding(0, 1)

	OuterBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(NeonMagenta).
			Padding(0, 1)

	// Branch styles for `sm log`
	BranchTrunk        = lipgloss.NewStyle().Foreground(RoleTrunk).Bold(true)
	BranchCurrent      = lipgloss.NewStyle().Foreground(RoleCurrentBranch).Bold(true)
	BranchHealthy      = lipgloss.NewStyle().Foreground(RoleHealthy)
	BranchNeedsRestack = lipgloss.NewStyle().Foreground(RoleNeedsRestack).Bold(true)
	BranchAncestor     = lipgloss.NewStyle().Foreground(RoleAncestor)

	// PR state pills
	PROpen   = lipgloss.NewStyle().Foreground(Black).Background(RolePROpen).Padding(0, 1).Bold(true)
	PRDraft  = lipgloss.NewStyle().Foreground(Black).Background(RolePRDraft).Padding(0, 1).Bold(true)
	PRMerged = lipgloss.NewStyle().Foreground(White).Background(RolePRMerged).Padding(0, 1).Bold(true)
	PRClosed = lipgloss.NewStyle().Foreground(Black).Background(RolePRClosed).Padding(0, 1)

	// CI status badges — rendered as a coloured chip next to the PR
	// pill in `sm log`. The label stays "CI" across all three states
	// so the column lines up; only the background colour shifts.
	// ChecksNone produces no badge so a config-less repo doesn't carry
	// a permanent grey chip per row.
	BadgeCIPass    = lipgloss.NewStyle().Foreground(Black).Background(NeonLime).Padding(0, 1).Bold(true)
	BadgeCIPending = lipgloss.NewStyle().Foreground(Black).Background(NeonYellow).Padding(0, 1).Bold(true)
	BadgeCIFail    = lipgloss.NewStyle().Foreground(White).Background(HotPink).Padding(0, 1).Bold(true)

	// Mergeability badges — appended after the CI badge when GitHub
	// reports a definitive mergeable / conflicting state. Drafts and
	// closed PRs intentionally produce no badge.
	BadgeMergeReady    = lipgloss.NewStyle().Foreground(Black).Background(NeonLime).Padding(0, 1).Bold(true)
	BadgeMergeConflict = lipgloss.NewStyle().Foreground(Black).Background(NeonOrange).Padding(0, 1).Bold(true)

	// Toasts
	SuccessToast = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(NeonLime).
			Foreground(NeonLime).
			Padding(0, 1).
			Bold(true)

	ErrorToast = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(HotPink).
			Foreground(HotPink).
			Padding(0, 1).
			Bold(true)

	// Help / key hints
	KeyHint  = lipgloss.NewStyle().Foreground(NeonMagenta).Bold(true)
	KeyLabel = lipgloss.NewStyle().Foreground(NeonCyan)
)
