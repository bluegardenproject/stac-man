package gh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Runner abstracts running a `gh` invocation. Production uses
// ExecRunner; tests inject a fake.
type Runner interface {
	Run(ctx context.Context, args ...string) (stdout string, stderr string, err error)
}

// ExecRunner shells out to the real `gh` binary in the current
// directory.
type ExecRunner struct {
	Dir string
}

// Run executes `gh args...` and returns stdout, stderr, and any
// non-zero exit error.
func (e ExecRunner) Run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if e.Dir != "" {
		cmd.Dir = e.Dir
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			err = fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
		} else {
			err = fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
		}
	}
	return outBuf.String(), errBuf.String(), err
}

// PreflightCheck verifies that the gh CLI is installed and the user is
// authenticated. Returns a friendly error message rather than the raw
// gh output so commands can surface it directly.
func PreflightCheck(ctx context.Context) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("the GitHub CLI (`gh`) is not installed or not on PATH; install it from https://cli.github.com/ and run `gh auth login`")
	}
	r := ExecRunner{}
	if _, _, err := r.Run(ctx, "auth", "status"); err != nil {
		return errors.New("`gh` is installed but not authenticated; run `gh auth login` first")
	}
	return nil
}
