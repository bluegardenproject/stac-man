// Package theme centralises colors, styles, and the gradient helper.
// It mirrors github-butler's theme package so the two CLIs feel like a
// family, and adds stack-aware role aliases (CurrentBranch, NeedsRestack,
// Trunk, …) that the rest of stac-man uses by intent rather than by
// raw color name.
package theme

import "github.com/charmbracelet/lipgloss"

// Neon palette (24-bit truecolor). Lipgloss falls back to the closest
// 256-color match on terminals that can't render truecolor.
var (
	NeonPink    = lipgloss.Color("#FF10F0")
	NeonCyan    = lipgloss.Color("#00F0FF")
	NeonMagenta = lipgloss.Color("#FF00FF")
	NeonLime    = lipgloss.Color("#39FF14")
	NeonPurple  = lipgloss.Color("#BF00FF")
	NeonOrange  = lipgloss.Color("#FF6A00")
	NeonYellow  = lipgloss.Color("#F5FF00")
	NeonBlue    = lipgloss.Color("#1B03FF")
	HotPink     = lipgloss.Color("#FF2A6D")

	Black  = lipgloss.Color("#000000")
	White  = lipgloss.Color("#FFFFFF")
	Dim    = lipgloss.Color("#6C6C80")
	DarkBg = lipgloss.Color("#120018")
)

// Stack-aware role aliases. Use these from rendering code so a future
// palette tweak only needs to change one place.
var (
	RoleCurrentBranch = NeonCyan    // the branch HEAD is on right now
	RoleTrunk         = NeonPurple  // the trunk branch (main / master)
	RoleNeedsRestack  = HotPink     // branch whose parent has moved
	RoleConflict      = NeonOrange  // interrupted-restack state
	RoleHealthy       = NeonLime    // clean / merged-ready
	RolePROpen        = NeonLime    // PR is open and not draft
	RolePRDraft       = NeonYellow  // PR is draft
	RolePRMerged      = NeonPurple  // PR is merged
	RolePRClosed      = Dim         // PR is closed without merging
	RoleAncestor      = Dim         // tree decoration / older nodes
	RoleAccent        = NeonMagenta // emphasized inline text
)

// Gradient stops (kept identical to github-butler so banners line up).
var (
	TitleStops      = []lipgloss.Color{NeonPink, NeonMagenta, NeonPurple, NeonCyan}
	HeaderStops     = []lipgloss.Color{NeonCyan, NeonPink}
	ConnectorStops  = []lipgloss.Color{NeonPurple, NeonMagenta, NeonPink, NeonCyan}
	CountdownStops  = []lipgloss.Color{NeonPink, NeonPurple, NeonCyan}
)
