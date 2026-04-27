package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Runner abstracts running a `git` invocation. Production uses
// ExecRunner; tests inject a fake that records arguments and returns
// canned output.
type Runner interface {
	Run(ctx context.Context, args ...string) (stdout string, stderr string, err error)
}

// ExecRunner shells out to the real `git` binary in the current working
// directory.
type ExecRunner struct {
	// Dir is the working directory to invoke git in. Empty string means
	// inherit from the parent process.
	Dir string
}

// command builds the *exec.Cmd that Run executes. Extracted so tests
// can assert the resulting argv, working directory, and env without
// actually invoking git.
//
// `sm` orchestrates git non-interactively, so we override GIT_EDITOR
// (and the legacy EDITOR fallback) to a no-op. Without this, paths
// like `git rebase --continue` — which internally run `git commit`
// to record a conflict resolution — fail with "Terminal is dumb,
// but EDITOR unset" the moment a user runs `sm` from any context
// without an interactive editor configured.
func (e ExecRunner) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	if e.Dir != "" {
		cmd.Dir = e.Dir
	}
	cmd.Env = append(os.Environ(), "GIT_EDITOR=:", "EDITOR=:")
	return cmd
}

// Run executes `git args...` and returns stdout, stderr, and any
// non-zero exit error. It does not parse output — that's the Client's
// job.
func (e ExecRunner) Run(ctx context.Context, args ...string) (string, string, error) {
	cmd := e.command(ctx, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		// Wrap with the exit message so callers don't have to type-assert.
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
		} else {
			err = fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
		}
	}
	return outBuf.String(), errBuf.String(), err
}

// ExitErrorOf returns the exit code from err if err wraps an
// *exec.ExitError; otherwise -1. Useful for callers that want to treat
// e.g. `git rev-parse` returning 128 as "no such ref" rather than a
// hard failure.
func ExitErrorOf(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}
