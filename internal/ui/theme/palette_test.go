package theme

import (
	"encoding/json"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestPaletteJSONLoaded asserts that the embedded palette.json parses and
// that every key the package depends on is present and well-formed. The
// production loader panics on failure, so reaching this test running at
// all already proves load succeeded — but we also re-parse defensively to
// catch a future change that swaps the loader for something silent.
func TestPaletteJSONLoaded(t *testing.T) {
	var p map[string]string
	if err := json.Unmarshal(paletteJSON, &p); err != nil {
		t.Fatalf("palette.json failed to parse: %v", err)
	}
	for _, k := range requiredKeys {
		v, ok := p[k]
		if !ok {
			t.Errorf("palette.json is missing required key %q", k)
			continue
		}
		if !isHexColor(v) {
			t.Errorf("palette.json key %q = %q is not a #RRGGBB hex color", k, v)
		}
	}
}

// TestNamedColorsMatchPalette locks the relationship between the public
// color vars exposed to other packages and the palette.json source. This
// is the contract that prevents drift if someone "fixes" a color in code
// without updating the JSON the docs site reads from.
func TestNamedColorsMatchPalette(t *testing.T) {
	cases := map[string]lipgloss.Color{
		"NeonPink":    NeonPink,
		"NeonCyan":    NeonCyan,
		"NeonMagenta": NeonMagenta,
		"NeonLime":    NeonLime,
		"NeonPurple":  NeonPurple,
		"NeonOrange":  NeonOrange,
		"NeonYellow":  NeonYellow,
		"NeonBlue":    NeonBlue,
		"HotPink":     HotPink,
		"Black":       Black,
		"White":       White,
		"Dim":         Dim,
		"DarkBg":      DarkBg,
	}
	for key, got := range cases {
		want := paletteMap[key]
		if string(got) != want {
			t.Errorf("color %s: package var = %q, palette.json = %q", key, string(got), want)
		}
	}
}

func TestIsHexColor(t *testing.T) {
	good := []string{"#000000", "#FFFFFF", "#39ff14", "#FF10F0"}
	for _, c := range good {
		if !isHexColor(c) {
			t.Errorf("isHexColor(%q) = false, want true", c)
		}
	}
	bad := []string{"", "#", "FF10F0", "#ZZZZZZ", "#FF10F", "#FF10F00", "rgb(0,0,0)"}
	for _, c := range bad {
		if isHexColor(c) {
			t.Errorf("isHexColor(%q) = true, want false", c)
		}
	}
}
