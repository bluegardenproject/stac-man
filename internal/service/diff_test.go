package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// serviceWithRunner builds a Service whose git.Client is wired to
// the given runner. Local helper because the existing test files
// inline the same construction site-by-site and CommitDiff's tests
// don't need any of the other variants.
func serviceWithRunner(t *testing.T, r git.Runner) *Service {
	t.Helper()
	return &Service{G: git.NewWithRunner(r), Store: memory.New()}
}

// commitDiffRunner returns a canned `git show` body when called
// with the right argv shape, and is otherwise minimally cooperative
// for the EnsureRepo preflight. Pulled out of snapshotRunner
// because CommitDiff's only branch decision is the early empty-sha
// guard, which is easier to read against a focused fake.
type commitDiffRunner struct {
	body string
	// lastArgs records the last invocation so tests can assert
	// the wrapper called `git show --no-color <sha>` rather than
	// some accidental variant.
	lastArgs []string
}

func (r *commitDiffRunner) Run(_ context.Context, args ...string) (string, string, error) {
	r.lastArgs = append([]string(nil), args...)
	if len(args) >= 2 && args[0] == "rev-parse" && args[1] == "--is-inside-work-tree" {
		return "true", "", nil
	}
	if len(args) >= 1 && args[0] == "show" {
		return r.body, "", nil
	}
	return "", "", nil
}

// TestCommitDiffPassesShaToGit pins the contract that the wrapper
// invokes `git show --no-color <sha>` and returns the body
// verbatim. The diff viewer relies on the unmodified body for its
// own colourisation pass — any rewriting here would corrupt hunk
// headers.
func TestCommitDiffPassesShaToGit(t *testing.T) {
	body := "commit deadbeef\nAuthor: Test\n\n    feat: do thing\n\ndiff --git a/foo b/foo\n@@ -1 +1 @@\n-old\n+new\n"
	r := &commitDiffRunner{body: body}
	svc := serviceWithRunner(t, r)

	got, err := svc.CommitDiff(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("CommitDiff: %v", err)
	}
	if got != body {
		t.Fatalf("CommitDiff body altered.\n got: %q\nwant: %q", got, body)
	}
	want := []string{"show", "--no-color", "deadbeef"}
	if len(r.lastArgs) != len(want) {
		t.Fatalf("git argv = %v, want %v", r.lastArgs, want)
	}
	for i, w := range want {
		if r.lastArgs[i] != w {
			t.Fatalf("argv[%d] = %q, want %q", i, r.lastArgs[i], w)
		}
	}
}

// TestCommitDiffEmptyShaErrors documents the input guard. An empty
// SHA reaching the git wrapper would degenerate into `git show`
// (which uses HEAD), masking a UI bug. Reject early instead.
func TestCommitDiffEmptyShaErrors(t *testing.T) {
	r := &commitDiffRunner{}
	svc := serviceWithRunner(t, r)

	_, err := svc.CommitDiff(context.Background(), "")
	if err == nil {
		t.Fatalf("CommitDiff(\"\") = nil err, want guard rejection")
	}
	if !strings.Contains(err.Error(), "empty sha") {
		t.Fatalf("error = %v, want 'empty sha' message", err)
	}
	if r.lastArgs != nil {
		t.Fatalf("git invoked despite empty sha: argv = %v", r.lastArgs)
	}
}
