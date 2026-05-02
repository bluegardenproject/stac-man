package service

import (
	"context"
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

// buildSubmitGraph builds a stack with this shape so the B7 tests
// can target several positions in the same graph without churning
// setup boilerplate:
//
//	main
//	└── feat-a
//	    └── feat-b
//	        └── feat-c
//	            └── feat-d
//
// prs maps branch names to PR numbers (0 means "no PR yet"). Any
// branch missing from prs gets PR=0.
func buildSubmitGraph(t *testing.T, prs map[string]int) *stack.Graph {
	t.Helper()
	s := memory.New()
	ctx := context.Background()
	if err := s.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	chain := []struct {
		name, parent string
	}{
		{"feat-a", "main"},
		{"feat-b", "feat-a"},
		{"feat-c", "feat-b"},
		{"feat-d", "feat-c"},
	}
	for _, b := range chain {
		if err := s.SetBranch(ctx, b.name, store.BranchMeta{Parent: b.parent, PR: prs[b.name]}); err != nil {
			t.Fatalf("SetBranch %s: %v", b.name, err)
		}
	}
	g, err := stack.Load(ctx, s)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	return g
}

func branchNames(targets []stack.Branch) []string {
	out := make([]string, len(targets))
	for i, b := range targets {
		out[i] = b.Name
	}
	return out
}

// TestSubmissionTargetsCurrentOnly pins the no-flag baseline: without
// --stack, only the current branch is processed. Pre-existing
// behaviour the new code must preserve.
func TestSubmissionTargetsCurrentOnly(t *testing.T) {
	g := buildSubmitGraph(t, nil)
	got := branchNames(submissionTargets(g, "feat-c", false))
	if want := []string{"feat-c"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackFromBottom pins the original `--stack`
// semantic: from the bottom of an unsubmitted stack, descendants
// follow naturally and the ancestor walk finds nothing to prepend.
func TestSubmissionTargetsStackFromBottom(t *testing.T) {
	g := buildSubmitGraph(t, nil)
	got := branchNames(submissionTargets(g, "feat-a", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackFromMiddleIncludesAncestors pins B7: a
// user running `sm submit --stack` from a non-bottom branch on a
// fresh stack still pushes the unsubmitted parents above. The
// resulting slice is trunk-toward-current order so the push loop
// opens parent PRs before child PRs.
func TestSubmissionTargetsStackFromMiddleIncludesAncestors(t *testing.T) {
	g := buildSubmitGraph(t, nil)
	got := branchNames(submissionTargets(g, "feat-c", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackFromTopIncludesAncestors pins the
// concrete shape the user hit in this repo: ran `sm submit --stack`
// from the leaf, only the leaf was pushed, PR creation failed
// because the base branch hadn't reached origin yet. With B7,
// every unsubmitted ancestor is now picked up automatically.
func TestSubmissionTargetsStackFromTopIncludesAncestors(t *testing.T) {
	g := buildSubmitGraph(t, nil)
	got := branchNames(submissionTargets(g, "feat-d", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackIncludesSubmittedAncestor pins B10:
// `--stack` no longer stops at the first PR'd ancestor. Earlier
// versions did, treating "PR exists" as "leave the ancestor alone";
// the live data in B10 showed that policy left mid-stack rewrites
// stranded on origin (any merge upstream cascades a restack to
// every descendant, so previously-submitted ancestors of `current`
// silently diverge from origin and break mergeability on GitHub
// the moment a leaf is re-pushed). Idempotency now lives in the
// push loop's `RemoteMatchesLocal` check, not in the target list.
func TestSubmissionTargetsStackIncludesSubmittedAncestor(t *testing.T) {
	// feat-a has PR #2 (already submitted, maybe rebased after a
	// merge upstream); feat-b is unsubmitted; feat-c is current.
	g := buildSubmitGraph(t, map[string]int{"feat-a": 2})
	got := branchNames(submissionTargets(g, "feat-c", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackIncludesSubmittedImmediateParent locks
// in B10's "always include all ancestors" rule even when the
// immediate parent already has a PR — the exact shape that used to
// truncate the target list to just `current` and its descendants.
func TestSubmissionTargetsStackIncludesSubmittedImmediateParent(t *testing.T) {
	g := buildSubmitGraph(t, map[string]int{"feat-b": 3})
	got := branchNames(submissionTargets(g, "feat-c", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestSubmissionTargetsStackEverythingPRd pins the most extreme
// re-run scenario: every branch already has a PR. With B10 we
// still target the whole stack, and the push loop is responsible
// for being a no-op when origin matches local.
func TestSubmissionTargetsStackEverythingPRd(t *testing.T) {
	g := buildSubmitGraph(t, map[string]int{
		"feat-a": 1, "feat-b": 2, "feat-c": 3, "feat-d": 4,
	})
	got := branchNames(submissionTargets(g, "feat-c", true))
	if want := []string{"feat-a", "feat-b", "feat-c", "feat-d"}; !equalStrings(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
}

// TestDivergedStackmatesReportsBranchesNotInTargets pins B10's
// plain-submit warning. From `feat-c`, plain submit only targets
// `feat-c` itself; if any other tracked branch's local tip differs
// from origin's tracking ref, the helper must surface it so the
// user knows to follow up with `sm submit --stack`.
func TestDivergedStackmatesReportsBranchesNotInTargets(t *testing.T) {
	g := buildSubmitGraph(t, map[string]int{"feat-a": 1, "feat-b": 2, "feat-c": 3, "feat-d": 4})
	// Drive RemoteMatchesLocal: feat-a is in sync; feat-b and
	// feat-d are diverged. feat-c is the current branch (already
	// in targets), so even if it diverged we wouldn't list it.
	r := &fakeRunner{
		responses: map[string]string{
			// feat-a: in sync.
			"rev-parse --verify feat-a^{commit}":                     "sha-a",
			"rev-parse --verify refs/remotes/origin/feat-a^{commit}": "sha-a",
			// feat-b: diverged.
			"rev-parse --verify feat-b^{commit}":                     "sha-b-local",
			"rev-parse --verify refs/remotes/origin/feat-b^{commit}": "sha-b-remote",
			// feat-c: in sync (but in targets so should not appear).
			"rev-parse --verify feat-c^{commit}":                     "sha-c",
			"rev-parse --verify refs/remotes/origin/feat-c^{commit}": "sha-c",
			// feat-d: diverged.
			"rev-parse --verify feat-d^{commit}":                     "sha-d-local",
			"rev-parse --verify refs/remotes/origin/feat-d^{commit}": "sha-d-remote",
		},
	}
	svc := &Service{G: git.NewWithRunner(r), Store: memory.New()}
	current := []stack.Branch{}
	if b, ok := g.Get("feat-c"); ok {
		current = []stack.Branch{b}
	}

	got := svc.divergedStackmates(context.Background(), g, "feat-c", current)
	wantNames := map[string]int{"feat-b": 2, "feat-d": 4}
	if len(got) != len(wantNames) {
		t.Fatalf("divergedStackmates = %#v, want %d entries (%v)", got, len(wantNames), wantNames)
	}
	for _, d := range got {
		pr, ok := wantNames[d.Branch]
		if !ok {
			t.Fatalf("unexpected branch %q in result", d.Branch)
		}
		if d.PR != pr {
			t.Fatalf("branch %q PR = %d, want %d", d.Branch, d.PR, pr)
		}
	}
}

// TestDivergedStackmatesIgnoresInSync pins the negative case: when
// every other branch is already in sync with origin, plain submit
// emits no warning.
func TestDivergedStackmatesIgnoresInSync(t *testing.T) {
	g := buildSubmitGraph(t, nil)
	r := &fakeRunner{fallback: "same-sha"}
	svc := &Service{G: git.NewWithRunner(r), Store: memory.New()}
	currentTarget, _ := g.Get("feat-c")
	got := svc.divergedStackmates(context.Background(), g, "feat-c", []stack.Branch{currentTarget})
	if len(got) != 0 {
		t.Fatalf("divergedStackmates = %#v, want empty", got)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

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

// TestClassifyStaleParentDefaultSkips pins the historical contract
// users coming from v2.0 rely on: a stale ParentSHA always blocks
// the push unless the user opts in via --no-restack.
func TestClassifyStaleParentDefaultSkips(t *testing.T) {
	if got := classifyStaleParent(false); got != staleSkip {
		t.Fatalf("classifyStaleParent(false) = %v, want staleSkip", got)
	}
}

// TestClassifyStaleParentNoRestackWarns pins the new --no-restack
// behaviour: the same stale ParentSHA now flows through to
// SubmitReport.StaleParentSHA instead of SubmitReport.Skipped, and
// the branch is pushed regardless.
func TestClassifyStaleParentNoRestackWarns(t *testing.T) {
	if got := classifyStaleParent(true); got != staleWarn {
		t.Fatalf("classifyStaleParent(true) = %v, want staleWarn", got)
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
