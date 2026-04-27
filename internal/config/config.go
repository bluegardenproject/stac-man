// Package config loads optional global preferences from
// ~/.config/stac-man/config.yaml. The file is entirely optional —
// stac-man works fine without it. Per-repo state (stack metadata,
// trunk) lives in git config, not here.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds user preferences. New keys should default sensibly so
// that an empty file still produces working defaults.
type Config struct {
	// Color enables ANSI color output. Defaults to "auto" (color when
	// stdout is a TTY and NO_COLOR is unset).
	Color string `yaml:"color,omitempty"`
	// DefaultBaseBranch overrides per-repo trunk detection when set.
	// Empty means "detect per repo".
	DefaultBaseBranch string `yaml:"default_base_branch,omitempty"`
	// PreferDraftPRs makes `sm submit` create draft PRs by default.
	PreferDraftPRs bool `yaml:"prefer_draft_prs,omitempty"`
}

// Default returns a Config with the documented defaults.
func Default() Config {
	return Config{
		Color:             "auto",
		DefaultBaseBranch: "",
		PreferDraftPRs:    false,
	}
}

// Path returns the canonical config file path. Honors $XDG_CONFIG_HOME
// when set, falls back to ~/.config/stac-man/config.yaml.
func Path() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "stac-man", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "stac-man", "config.yaml"), nil
}

// Load reads the config file at Path(). A missing file is not an
// error — the defaults are returned instead. Parse errors ARE
// surfaced so users notice typos.
func Load() (Config, error) {
	cfg := Default()
	p, err := Path()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", p, err)
	}
	return cfg, nil
}
