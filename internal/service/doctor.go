package service

import (
	"context"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/stack"
)

// DoctorReport summarizes the health of stac-man's metadata vs. the
// actual git state. An empty Issues slice means everything is healthy.
type DoctorReport struct {
	Trunk          string
	TrackedCount   int
	NeedsRestack   []string // branches whose ParentSHA is stale
	StaleSHA       []string // branches whose recorded parent commit doesn't exist
	UntrackedRoots []string // local branches that look like stack roots but aren't tracked
	Issues         []string // graph-level errors (cycles, missing parents)
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
		// Verify the recorded parent SHA still exists.
		if b.ParentSHA != "" {
			if _, err := s.G.RevParse(ctx, b.ParentSHA); err != nil {
				r.StaleSHA = append(r.StaleSHA, fmt.Sprintf("%s (recorded parent SHA %s no longer exists)", b.Name, short(b.ParentSHA)))
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

	return r, nil
}

func short(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
