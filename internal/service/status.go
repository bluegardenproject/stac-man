package service

import (
	"context"

	"github.com/bluegardenproject/stac-man/internal/cache"
	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/stack"
)

type prStatusClient interface {
	PRStatusForNumber(ctx context.Context, number int) (gh.PRStatus, error)
}

var (
	statusPreflightCheck = gh.PreflightCheck
	newStatusClient      = func() prStatusClient { return gh.New("") }
)

// StatusReport is the live GitHub status snapshot for the tracked PRs
// in the current stack.
type StatusReport struct {
	Trunk    string
	Branches []StatusBranch
}

// StatusBranch is one tracked branch with a recorded PR number. Err is
// set when GitHub status could not be fetched for that PR; other rows
// continue so the command can show partial progress.
type StatusBranch struct {
	Branch    string
	PR        int
	State     gh.PRState
	Draft     bool
	Checks    gh.CheckRollup
	Mergeable gh.Mergeability
	Err       string
}

// Status fetches live GitHub checks and mergeability for tracked PRs,
// persists the results in the SQLite cache, and returns a renderable
// report. Unlike Log, this command intentionally pays the GitHub cost.
func (s *Service) Status(ctx context.Context) (StatusReport, error) {
	r := StatusReport{}
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
	if err := statusPreflightCheck(ctx); err != nil {
		return r, err
	}

	client := newStatusClient()
	for _, b := range g.Branches() {
		if b.PR == 0 {
			continue
		}
		row := StatusBranch{Branch: b.Name, PR: b.PR}
		status, err := client.PRStatusForNumber(ctx, b.PR)
		if err != nil {
			row.Err = err.Error()
			r.Branches = append(r.Branches, row)
			continue
		}
		row.State = status.State
		row.Draft = status.IsDraft
		row.Checks = status.Checks
		row.Mergeable = status.Mergeable
		r.Branches = append(r.Branches, row)
		s.persistPRStatusSnapshot(ctx, status)
	}

	return r, nil
}

func (s *Service) persistPRStatusSnapshot(ctx context.Context, st gh.PRStatus) {
	if st.Number == 0 {
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
	_ = db.PutPRStatusSnapshot(ctx, cache.PRStatusSnapshot{
		Number:    st.Number,
		Checks:    string(st.Checks),
		Mergeable: string(st.Mergeable),
		State:     string(st.State),
		Draft:     st.IsDraft,
	})
}

func (s *Service) refreshCachedPRStatuses(ctx context.Context, g *stack.Graph, client prStatusClient) {
	if g == nil || client == nil {
		return
	}
	for _, b := range g.Branches() {
		if b.PR == 0 {
			continue
		}
		status, err := client.PRStatusForNumber(ctx, b.PR)
		if err != nil {
			continue
		}
		s.persistPRStatusSnapshot(ctx, status)
	}
}
