package service

import (
	"context"
	"strings"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/memory"
)

// TestDoctorReportsDriftedParent reproduces the second bug found
// during manual testing: a recorded parent SHA still exists somewhere
// in the repo (so the existing StaleSHA check passes), but it's no
// longer reachable from the branch's own history because a stray
// `git commit --amend` (or `sm modify` before B1 was fixed) rewrote
// the chain. Doctor must flag this — otherwise descendants would be
// rebased against a phantom parent.
func TestDoctorReportsDriftedParent(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]string{
			"rev-parse --is-inside-work-tree": "true",
			// Parent commit still exists in the repo (RevParse succeeds).
			"rev-parse --verify abc123^{commit}": "abc123",
			// Parent branch tip equals the recorded ParentSHA, so the
			// existing "needs restack" heuristic stays silent.
			"rev-parse --verify base^{commit}": "abc123",
			// merge-base --is-ancestor exits non-zero → not an ancestor.
			"merge-base --is-ancestor abc123 feat-rewritten":     "",
			"for-each-ref --format=%(refname:short) refs/heads/": "main\nbase\nfeat-rewritten\n",
		},
		// Default: rev-list for "untracked roots" returns 0 ahead.
		fallback: "0",
	}
	// merge-base --is-ancestor: success → ancestor; non-zero exit 1 → not ancestor.
	// We can't easily inject an exec.ExitError here, so widen the fake
	// to also implement an exit-1 path for the is-ancestor lookup.
	r.responses["merge-base --is-ancestor abc123 feat-rewritten"] = "<not-ancestor>"
	r.errResponses = map[string]error{
		"merge-base --is-ancestor abc123 feat-rewritten": exitOne,
	}

	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "base", store.BranchMeta{Parent: "main", ParentSHA: "rootSHA"}); err != nil {
		t.Fatalf("SetBranch base: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-rewritten", store.BranchMeta{Parent: "base", ParentSHA: "abc123"}); err != nil {
		t.Fatalf("SetBranch rewritten: %v", err)
	}
	// Stub the ones we don't want to hit drift logic for (the "base"
	// branch's recorded parent SHA must look healthy).
	r.responses["rev-parse --verify rootSHA^{commit}"] = "rootSHA"
	r.responses["rev-parse --verify main^{commit}"] = "rootSHA"
	r.responses["merge-base --is-ancestor rootSHA base"] = "ok"
	// And the "untracked roots" sniff: every non-tracked local branch is
	// asked CountCommitsAhead vs. trunk; main is the trunk so skip; only
	// non-tracked branch in our list is the empty set since base &
	// feat-rewritten are tracked.

	s := &Service{G: git.NewWithRunner(r), Store: mem}
	report, err := s.Doctor(ctx)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}

	if len(report.DriftedParent) != 1 {
		t.Fatalf("expected 1 drifted parent, got %d: %+v", len(report.DriftedParent), report)
	}
	if !strings.Contains(report.DriftedParent[0], "feat-rewritten") {
		t.Fatalf("expected drift for feat-rewritten, got %q", report.DriftedParent[0])
	}
	if len(report.NeedsRestack) != 0 {
		t.Fatalf("did not expect NeedsRestack here, got %v", report.NeedsRestack)
	}
	if len(report.StaleSHA) != 0 {
		t.Fatalf("did not expect StaleSHA here (commit still exists), got %v", report.StaleSHA)
	}
}

// TestDoctorHealthyChainProducesNoDrift is the happy-path counterpart:
// when the recorded parent SHA is reachable from the branch tip, the
// new check must stay silent.
func TestDoctorHealthyChainProducesNoDrift(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]string{
			"rev-parse --is-inside-work-tree":                    "true",
			"rev-parse --verify abc123^{commit}":                 "abc123",
			"rev-parse --verify base^{commit}":                   "abc123",
			"rev-parse --verify main^{commit}":                   "rootSHA",
			"rev-parse --verify rootSHA^{commit}":                "rootSHA",
			"merge-base --is-ancestor abc123 feat-clean":         "ok",
			"merge-base --is-ancestor rootSHA base":              "ok",
			"for-each-ref --format=%(refname:short) refs/heads/": "main\nbase\nfeat-clean\n",
		},
		fallback: "0",
	}

	mem := memory.New()
	ctx := context.Background()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "base", store.BranchMeta{Parent: "main", ParentSHA: "rootSHA"}); err != nil {
		t.Fatalf("SetBranch base: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-clean", store.BranchMeta{Parent: "base", ParentSHA: "abc123"}); err != nil {
		t.Fatalf("SetBranch clean: %v", err)
	}

	s := &Service{G: git.NewWithRunner(r), Store: mem}
	report, err := s.Doctor(ctx)
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if len(report.DriftedParent) != 0 {
		t.Fatalf("expected no drift, got %v", report.DriftedParent)
	}
}
