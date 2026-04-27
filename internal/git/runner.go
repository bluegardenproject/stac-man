package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// Run executes `git args...` and returns stdout, stderr, and any
// non-zero exit error. It does not parse output — that's the Client's
// job.
func (e ExecRunner) Run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if e.Dir != "" {
		cmd.Dir = e.Dir
	}
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
