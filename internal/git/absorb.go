package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// HasGitAbsorb reports whether `git-absorb` is on PATH. `git absorb` is
// resolved by `git` as a subcommand when the binary is named
// `git-absorb`, so we can't just `git help absorb` — we have to check
// the executable directly.
func HasGitAbsorb() bool {
	_, err := exec.LookPath("git-absorb")
	return err == nil
}

// ErrAbsorbMissing is returned when `git-absorb` is not installed.
// The CLI layer turns this into a friendly install hint.
var ErrAbsorbMissing = errors.New("git-absorb is not installed")

// Absorb runs `git absorb --base <baseRef> --and-rebase=false`. We
// keep the rebase flag off so stac-man's own restack engine controls
// the descendant cascade. baseRef can be a SHA or a branch name; passing
// the parent's tip SHA is the most reliable in our flows because the
// caller has already resolved it.
//
// Returns ErrAbsorbMissing if the binary isn't installed.
func (c *Client) Absorb(ctx context.Context, baseRef string) error {
	if !HasGitAbsorb() {
		return ErrAbsorbMissing
	}
	if baseRef == "" {
		return fmt.Errorf("absorb: base ref is required")
	}
	_, _, err := c.r.Run(ctx, "absorb", "--base", baseRef, "--and-rebase=false")
	return err
}
