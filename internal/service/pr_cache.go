package service

import (
	"context"
	"time"

	"github.com/bluegardenproject/stac-man/internal/cache"
	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/stack"
)

func (s *Service) cachedPRs(ctx context.Context, branches []stack.Branch) map[string]gh.PR {
	out := map[string]gh.PR{}
	if len(branches) == 0 {
		return out
	}
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return out
	}
	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		return out
	}
	defer db.Close()

	names := make([]string, 0, len(branches))
	for _, b := range branches {
		names = append(names, b.Name)
	}
	snaps, err := db.PRSnapshots(ctx, names)
	if err != nil {
		return out
	}
	for branch, snap := range snaps {
		out[branch] = gh.PR{
			Number:  snap.Number,
			Title:   snap.Title,
			State:   gh.PRState(snap.State),
			IsDraft: snap.Draft,
			URL:     snap.URL,
			Base:    snap.Base,
			Head:    snap.Head,
		}
	}
	return out
}

func (s *Service) cachedPRStatusBranches(ctx context.Context, items []CheckoutItem) map[string]StatusBranch {
	out := map[string]StatusBranch{}
	if len(items) == 0 {
		return out
	}
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return out
	}
	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		return out
	}
	defer db.Close()

	for _, it := range items {
		if it.PR == 0 {
			continue
		}
		snap, ok, err := db.PRStatusSnapshot(ctx, it.PR)
		if err != nil || !ok {
			continue
		}
		out[it.Branch] = StatusBranch{
			Branch:    it.Branch,
			PR:        it.PR,
			Checks:    gh.CheckRollup(snap.Checks),
			Mergeable: gh.Mergeability(snap.Mergeable),
			State:     gh.PRState(snap.State),
			Draft:     snap.Draft,
		}
	}
	return out
}

func (s *Service) persistPRSnapshot(ctx context.Context, branch string, pr gh.PR) {
	if branch == "" || pr.Number == 0 {
		return
	}
	s.persistPRSnapshotAt(ctx, branch, pr, time.Time{})
}

func (s *Service) persistPRSnapshotAt(ctx context.Context, branch string, pr gh.PR, fetchedAt time.Time) {
	if branch == "" || pr.Number == 0 {
		return
	}
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return
	}
	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		return
	}
	defer db.Close()
	_ = db.PutPRSnapshot(ctx, cache.PRSnapshot{
		Branch:    branch,
		Number:    pr.Number,
		State:     string(pr.State),
		Draft:     pr.IsDraft,
		Title:     pr.Title,
		URL:       pr.URL,
		Base:      pr.Base,
		Head:      pr.Head,
		FetchedAt: fetchedAt,
	})
}

func (s *Service) persistPRSnapshots(ctx context.Context, prs map[string]gh.PR) {
	fetchedAt := time.Now().UTC()
	for branch, pr := range prs {
		s.persistPRSnapshotAt(ctx, branch, pr, fetchedAt)
	}
}

func (s *Service) invalidatePRStatusSnapshots(ctx context.Context, numbers []int) {
	gitDir, err := s.G.GitDir(ctx)
	if err != nil {
		return
	}
	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		return
	}
	defer db.Close()
	_ = db.DeletePRStatusSnapshots(ctx, numbers)
}
