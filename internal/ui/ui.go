package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
)

// ColorEnabled reports whether stac-man should emit ANSI color. Honors
// the standard NO_COLOR convention, the cobra root's --no-color flag
// (which sets NO_COLOR=1 in PersistentPreRunE), and the absence of a
// TTY when stdout is piped or redirected.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if fi, err := os.Stdout.Stat(); err == nil {
		if (fi.Mode() & os.ModeCharDevice) == 0 {
			return false
		}
	}
	return true
}

// Banner renders the synthwave gradient title used at the top of long
// outputs (sm log, sm doctor). When color is disabled it returns the
// raw string so screenshots and pipes stay clean.
func Banner(s string) string {
	if !ColorEnabled() {
		return s
	}
	return theme.Gradient(s, theme.TitleStops...)
}

// Render runs s through style only when color is enabled. This is the
// single chokepoint every command should use so we never have to
// sprinkle NO_COLOR checks across the codebase.
func Render(style lipgloss.Style, s string) string {
	if !ColorEnabled() {
		return s
	}
	return style.Render(s)
}
