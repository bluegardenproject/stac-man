// Package theme centralises colors, styles, and the gradient helper.
// It mirrors github-butler's theme package so the two CLIs feel like a
// family, and adds stack-aware role aliases (CurrentBranch, NeedsRestack,
// Trunk, …) that the rest of stac-man uses by intent rather than by
// raw color name.
//
// The brand palette lives in palette.json next to this file. That same
// file is consumed by the docs site under docs/site/, so the Go TUI and
// the website can never drift on colors. palette.json is the single
// source-of-truth; if you need to tweak a color, edit it there.
package theme

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

//go:embed palette.json
var paletteJSON []byte

// paletteMap is loaded from palette.json at package load. It's a package
// variable (not an init() side effect) so the named color vars below can
// depend on it directly — Go orders package-level var initialization by
// dependency, so the styles in styles.go also see correct values.
var paletteMap = mustLoadPalette()

// requiredKeys lists every key the rest of this package reads. Loading
// fails loudly if any are missing or malformed, so a typo in palette.json
// surfaces at startup rather than as an unstyled rendering bug.
var requiredKeys = []string{
	"NeonPink", "NeonCyan", "NeonMagenta", "NeonLime", "NeonPurple",
	"NeonOrange", "NeonYellow", "NeonBlue", "HotPink",
	"Black", "White", "Dim", "DarkBg",
}

func mustLoadPalette() map[string]string {
	var p map[string]string
	if err := json.Unmarshal(paletteJSON, &p); err != nil {
		panic(fmt.Sprintf("theme: invalid palette.json: %v", err))
	}
	for _, k := range requiredKeys {
		v, ok := p[k]
		if !ok {
			panic("theme: palette.json missing key " + k)
		}
		if !isHexColor(v) {
			panic(fmt.Sprintf("theme: palette.json key %s is not a #RRGGBB hex color (got %q)", k, v))
		}
	}
	return p
}

// Neon palette (24-bit truecolor). Lipgloss falls back to the closest
// 256-color match on terminals that can't render truecolor.
var (
	NeonPink    = lipgloss.Color(paletteMap["NeonPink"])
	NeonCyan    = lipgloss.Color(paletteMap["NeonCyan"])
	NeonMagenta = lipgloss.Color(paletteMap["NeonMagenta"])
	NeonLime    = lipgloss.Color(paletteMap["NeonLime"])
	NeonPurple  = lipgloss.Color(paletteMap["NeonPurple"])
	NeonOrange  = lipgloss.Color(paletteMap["NeonOrange"])
	NeonYellow  = lipgloss.Color(paletteMap["NeonYellow"])
	NeonBlue    = lipgloss.Color(paletteMap["NeonBlue"])
	HotPink     = lipgloss.Color(paletteMap["HotPink"])

	Black  = lipgloss.Color(paletteMap["Black"])
	White  = lipgloss.Color(paletteMap["White"])
	Dim    = lipgloss.Color(paletteMap["Dim"])
	DarkBg = lipgloss.Color(paletteMap["DarkBg"])
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
	TitleStops     = []lipgloss.Color{NeonPink, NeonMagenta, NeonPurple, NeonCyan}
	HeaderStops    = []lipgloss.Color{NeonCyan, NeonPink}
	ConnectorStops = []lipgloss.Color{NeonPurple, NeonMagenta, NeonPink, NeonCyan}
	CountdownStops = []lipgloss.Color{NeonPink, NeonPurple, NeonCyan}
)

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		case c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}
