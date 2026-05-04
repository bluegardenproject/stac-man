package config

import (
	"os"
	"path/filepath"
	"testing"
)

// withFakeXDG points XDG_CONFIG_HOME at a temp dir for the duration of
// the test, so we never read or write the developer's real config.
func withFakeXDG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	withFakeXDG(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned %v on missing file; want nil", err)
	}
	if got, want := cfg, Default(); got != want {
		t.Fatalf("Load on missing file = %+v; want defaults %+v", got, want)
	}
}

func TestLoad_RoundTripsValuesFromFile(t *testing.T) {
	xdg := withFakeXDG(t)

	dir := filepath.Join(xdg, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("color: never\nprefer_draft_prs: true\ndefault_base_branch: develop\n")
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Color != ColorNever {
		t.Errorf("Color = %q; want %q", cfg.Color, ColorNever)
	}
	if !cfg.PreferDraftPRs {
		t.Errorf("PreferDraftPRs = false; want true")
	}
	if cfg.DefaultBaseBranch != "develop" {
		t.Errorf("DefaultBaseBranch = %q; want %q", cfg.DefaultBaseBranch, "develop")
	}
}

func TestLoad_RejectsInvalidColor(t *testing.T) {
	xdg := withFakeXDG(t)

	dir := filepath.Join(xdg, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("color: rainbow\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("Load with color=rainbow returned nil; want validation error")
	}
}

func TestLoad_SurfacesYAMLParseErrors(t *testing.T) {
	xdg := withFakeXDG(t)

	dir := filepath.Join(xdg, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Unbalanced quotes — invalid YAML.
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("color: \"never\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Fatal("Load on malformed YAML returned nil; want parse error")
	}
}

func TestSave_AndInitAreIdempotent(t *testing.T) {
	xdg := withFakeXDG(t)

	p, created, err := Init()
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !created {
		t.Fatal("Init reported created=false on first call")
	}
	if want := filepath.Join(xdg, "stac-man", "config.yaml"); p != want {
		t.Errorf("Init path = %q; want %q", p, want)
	}

	// Second call must not overwrite — idempotent by contract.
	p2, created2, err := Init()
	if err != nil {
		t.Fatalf("Init second call: %v", err)
	}
	if created2 {
		t.Fatal("Init reported created=true on second call (should preserve existing file)")
	}
	if p2 != p {
		t.Errorf("Init path changed between calls: %q vs %q", p, p2)
	}

	// Round-trip a non-default value through Save → Load.
	mutated := Default()
	mutated.PreferDraftPRs = true
	if _, err := Save(mutated); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("Load after Save: %v", err)
	}
	if !got.PreferDraftPRs {
		t.Errorf("Load did not see PreferDraftPRs=true after Save")
	}
}

func TestPath_HonorsXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-fake")

	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/xdg-fake/stac-man/config.yaml"; p != want {
		t.Errorf("Path = %q; want %q", p, want)
	}
}
