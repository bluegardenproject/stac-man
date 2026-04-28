package service

import (
	"context"
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// TestDerivePRMetaSingleCommit pins B6: a branch with exactly one
// commit produces a PR whose title is the commit subject and whose
// body is the commit body. This is the common case for stacked PRs
// — every fix branch in this repo's recent stack carried a single
// commit, and the pre-fix `sm submit` produced empty bodies and
// branch-name-derived titles.
func TestDerivePRMetaSingleCommit(t *testing.T) {
	const sha = "deadbeefcafebabedeadbeefcafebabedeadbeef"
	r := &fakeRunner{
		responses: map[string]string{
			"log --reverse --pretty=format:%H%x09%s main..feat-x": sha + "\tfeat(x): make the thing work",
			"log -1 --format=%s%x00%b " + sha:                     "feat(x): make the thing work\x00The body explains why this matters.\n\nMultiple lines are fine.\n",
		},
	}
	s := &Service{G: git.NewWithRunner(r), Store: memory.New()}

	title, body := s.derivePRMeta(context.Background(), "feat-x", "main")

	if want := "feat(x): make the thing work"; title != want {
		t.Fatalf("title = %q, want %q", title, want)
	}
	if !strings.Contains(body, "The body explains why this matters.") {
		t.Fatalf("body missing first paragraph: %q", body)
	}
	if !strings.Contains(body, "Multiple lines are fine.") {
		t.Fatalf("body missing second paragraph: %q", body)
	}
}

// TestDerivePRMetaMultipleCommits pins the multi-commit branch case:
// title is the FIRST (oldest) commit's subject (the one that defines
// the branch), body is a bulleted list of every subject so reviewers
// can see the shape of the change at a glance.
func TestDerivePRMetaMultipleCommits(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]string{
			"log --reverse --pretty=format:%H%x09%s main..feat-x": strings.Join([]string{
				"aaaa\tfeat(x): scaffold the thing",
				"bbbb\ttest(x): cover the happy path",
				"cccc\tdocs(x): note the trade-offs",
			}, "\n"),
		},
	}
	s := &Service{G: git.NewWithRunner(r), Store: memory.New()}

	title, body := s.derivePRMeta(context.Background(), "feat-x", "main")

	if want := "feat(x): scaffold the thing"; title != want {
		t.Fatalf("title = %q, want %q", title, want)
	}
	wantLines := []string{
		"Commits in this PR:",
		"- feat(x): scaffold the thing",
		"- test(x): cover the happy path",
		"- docs(x): note the trade-offs",
	}
	for _, line := range wantLines {
		if !strings.Contains(body, line) {
			t.Fatalf("body missing %q:\n%s", line, body)
		}
	}
	// CommitFullMessage must NOT have run — multi-commit branches use
	// the bullet list, not the first commit's body.
	if r.called("log", "-1", "--format=%s%x00%b") {
		t.Fatalf("CommitFullMessage shouldn't run for multi-commit branches; calls = %v", r.calls)
	}
}

// TestDerivePRMetaEmptyChain pins the fallback contract: when the
// branch has no commits (degenerate case — empty branch tip, or
// LogBetween errored), derivePRMeta returns empty strings and the
// caller falls back to buildPRTitle / opts.Body. We pre-fix this
// case wasn't reached because Submit always used buildPRTitle, but
// post-fix we depend on the empty-string contract.
func TestDerivePRMetaEmptyChain(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]string{
			"log --reverse --pretty=format:%H%x09%s main..feat-x": "",
		},
	}
	s := &Service{G: git.NewWithRunner(r), Store: memory.New()}

	title, body := s.derivePRMeta(context.Background(), "feat-x", "main")
	if title != "" || body != "" {
		t.Fatalf("expected empty title/body for empty chain, got %q / %q", title, body)
	}
}

// TestDerivePRMetaSingleCommitNoBody pins the "subject only" commit
// case: a commit with no body should produce a non-empty title and
// an empty body, so the non-destructive update path in Submit (which
// only sets body when derivedBody != "") leaves the existing empty
// body alone instead of churning the PR.
func TestDerivePRMetaSingleCommitNoBody(t *testing.T) {
	const sha = "abcd1234abcd1234abcd1234abcd1234abcd1234"
	r := &fakeRunner{
		responses: map[string]string{
			"log --reverse --pretty=format:%H%x09%s main..feat-x": sha + "\tfix: the obvious thing",
			"log -1 --format=%s%x00%b " + sha:                     "fix: the obvious thing\x00",
		},
	}
	s := &Service{G: git.NewWithRunner(r), Store: memory.New()}

	title, body := s.derivePRMeta(context.Background(), "feat-x", "main")
	if title != "fix: the obvious thing" {
		t.Fatalf("title = %q", title)
	}
	if body != "" {
		t.Fatalf("body should be empty for subject-only commit, got %q", body)
	}
}
