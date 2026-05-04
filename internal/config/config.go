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

// ColorMode values accepted in the `color:` key.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
)

// Config holds user preferences. New keys should default sensibly so
// that an empty file still produces working defaults.
type Config struct {
	// Color enables ANSI color output. One of "auto" (default — color
	// when stdout is a TTY and NO_COLOR is unset), "always", "never".
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
		Color:             ColorAuto,
		DefaultBaseBranch: "",
		PreferDraftPRs:    false,
	}
}

// Validate checks that the loaded values are within their allowed
// sets. Returns nil for the zero / default config.
func (c Config) Validate() error {
	switch c.Color {
	case "", ColorAuto, ColorAlways, ColorNever:
	default:
		return fmt.Errorf("invalid color %q (want one of: auto, always, never)", c.Color)
	}
	return nil
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
// error — the defaults are returned instead. Parse and validation
// errors ARE surfaced so users notice typos.
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
	if err := cfg.Validate(); err != nil {
		return cfg, fmt.Errorf("%s: %w", p, err)
	}
	return cfg, nil
}

// Save writes cfg to Path() as YAML, creating parent directories as
// needed. Existing files are overwritten atomically (temp + rename).
func Save(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	p, err := Path()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".config.*.yaml")
	if err != nil {
		return "", err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		_ = os.Remove(tmp.Name())
		return "", err
	}
	return p, nil
}

// Init writes a default config file if one does not already exist.
// Returns the path written to and a bool indicating whether a new file
// was created (false if the file already existed and was left alone).
func Init() (string, bool, error) {
	p, err := Path()
	if err != nil {
		return "", false, err
	}
	if _, err := os.Stat(p); err == nil {
		return p, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, false, err
	}
	written, err := Save(Default())
	return written, err == nil, err
}
