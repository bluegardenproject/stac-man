package service

import (
	"context"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/stack"
)

// DoctorReport summarizes the health of stac-man's metadata vs. the
// actual git state. An empty Issues slice means everything is healthy.
type DoctorReport struct {
	Trunk          string
	TrackedCount   int
	NeedsRestack   []string        // branches whose ParentSHA is stale
	StaleSHA       []string        // branches whose recorded parent commit doesn't exist
	DriftedParent  []string        // recorded parent SHA exists but is not in the branch's history
	UntrackedRoots []string        // local branches that look like stack roots but aren't tracked
	MergeConflicts []MergeConflict // PRs GitHub reports as CONFLICTING
	Issues         []string        // graph-level errors (cycles, missing parents)
}

// MergeConflict identifies a tracked branch whose PR is reported as
// CONFLICTING by GitHub. The doctor surfaces these as a louder block
// than `sm log`'s ⚠ glyph so the user has to acknowledge the
// conflict before continuing.
type MergeConflict struct {
	Branch string
	PR     int
}

// Doctor sanity-checks stac-man's metadata against the git working
// state without mutating anything. The cmd layer prints what comes
// back; this method itself is read-only.
func (s *Service) Doctor(ctx context.Context) (DoctorReport, error) {
	r := DoctorReport{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	r.Trunk = trunk

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	tracked := g.Branches()
	r.TrackedCount = len(tracked)

	local, err := s.G.LocalBranches(ctx)
	if err != nil {
		return r, err
	}
	for _, e := range g.Validate(local) {
		r.Issues = append(r.Issues, e.Error())
	}

	for _, b := range tracked {
		if b.Parent == "" {
			continue
		}
		// Verify the recorded parent SHA still exists, and — if it does —
		// that it's actually reachable from this branch's tip. The latter
		// catches the case where a misuse of `git commit --amend` (or an
		// external tool) rewrites history so the recorded SHA still
		// exists somewhere in the repo but no longer in *this* branch.
		if b.ParentSHA != "" {
			if _, err := s.G.RevParse(ctx, b.ParentSHA); err != nil {
				r.StaleSHA = append(r.StaleSHA, fmt.Sprintf("%s (recorded parent SHA %s no longer exists)", b.Name, short(b.ParentSHA)))
			} else if ok, err := s.G.IsAncestor(ctx, b.ParentSHA, b.Name); err == nil && !ok {
				r.DriftedParent = append(r.DriftedParent, fmt.Sprintf("%s (recorded parent SHA %s is not in this branch's history)", b.Name, short(b.ParentSHA)))
			}
		}
		// Detect stale ParentSHA: parent's tip has moved.
		tip, err := s.G.RevParse(ctx, b.Parent)
		if err == nil && tip != b.ParentSHA {
			r.NeedsRestack = append(r.NeedsRestack, b.Name)
		}
	}

	// Local branches that look like roots (parent = trunk by merge-base)
	// but aren't tracked at all. We don't auto-track because that's a
	// destructive guess; just flag them.
	trackedSet := map[string]bool{trunk: true}
	for _, b := range tracked {
		trackedSet[b.Name] = true
	}
	for _, name := range local {
		if trackedSet[name] {
			continue
		}
		// Sniff: branch has commits that aren't in trunk?
		ahead, err := s.G.CountCommitsAhead(ctx, name, trunk)
		if err == nil && ahead > 0 {
			r.UntrackedRoots = append(r.UntrackedRoots, name)
		}
	}

	r.MergeConflicts = s.detectMergeConflicts(ctx, tracked)

	return r, nil
}

// detectMergeConflicts asks GitHub for the mergeability of each
// tracked branch with a PR and returns the ones GitHub flags as
// CONFLICTING. The check is best-effort: gh missing, no auth, or
// parse failures collapse to "no conflicts surfaced" rather than
// failing the whole doctor run, because doctor must always work
// offline (its primary purpose is local-metadata sanity).
//
// We deliberately skip drafts here too — a draft PR can sit in
// CONFLICTING for weeks without being actionable, and the explicit
// doctor block is meant to flag PRs the user is preparing to land.
func (s *Service) detectMergeConflicts(ctx context.Context, tracked []stack.Branch) []MergeConflict {
	prMap := map[string]gh.PR{}
	for _, b := range tracked {
		if b.PR == 0 {
			continue
		}
		// PRStateOpen as a placeholder so fetchPRStatuses doesn't
		// short-circuit on closed/merged. Real state comes back in
		// the gh.PRStatus we receive.
		prMap[b.Name] = gh.PR{Number: b.PR, State: gh.PRStateOpen}
	}
	if len(prMap) == 0 {
		return nil
	}
	statuses := s.fetchPRStatuses(ctx, prMap, statusFetchOptions{wantMerge: true})
	var out []MergeConflict
	for _, b := range tracked {
		st, ok := statuses[b.Name]
		if !ok {
			continue
		}
		if st.IsDraft {
			continue
		}
		if st.State == gh.PRStateMerged || st.State == gh.PRStateClosed {
			continue
		}
		if st.Mergeable != gh.MergeConflicting {
			continue
		}
		out = append(out, MergeConflict{Branch: b.Name, PR: b.PR})
	}
	return out
}

func short(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
