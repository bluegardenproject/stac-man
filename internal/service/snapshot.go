package service

import (
	"context"

	"github.com/bluegardenproject/stac-man/internal/restack"
	"github.com/bluegardenproject/stac-man/internal/stack"
)

// PausedSnapshot is the interactive cockpit's view of a paused
// stac-man operation. Aliased to restack.PausedInfo so the engine
// stays the single source of truth for the paused-state shape;
// adding a field on PausedInfo automatically propagates to the
// dashboard, the resolver screen, and any future agent-facing
// status command without a translation layer.
type PausedSnapshot = restack.PausedInfo

// DashboardSnapshot is the atomic read the interactive cockpit takes
// whenever it needs to refresh. Bundling Trunk + Current + Log +
// CheckoutItems + Paused in one method means the rendered UI cannot
// disagree with itself across panes — every refresh comes from a
// single graph load, a single git HEAD read, and a single paused-
// state lookup.
type DashboardSnapshot struct {
	Trunk         string
	Current       string
	Log           *LogResult
	CheckoutItems []CheckoutItem
	GitHubStatus  map[string]StatusBranch
	Paused        *PausedSnapshot
}

// Snapshot returns the dashboard payload in one call. logOpts is
// forwarded to LogData, so callers control whether PR / CI / merge-
// status fetches happen on this refresh; pass a zero LogOptions for
// the fastest local-only read (the typical cockpit auto-refresh).
//
// Errors from the paused-state lookup are swallowed only when the
// state file is absent; a present-but-corrupt file or a failure to
// list conflict paths surfaces as an error so the cockpit can show
// a real diagnostic rather than silently hide a stuck restack.
func (s *Service) Snapshot(ctx context.Context, logOpts LogOptions) (*DashboardSnapshot, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return nil, err
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return nil, err
	}
	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		// Detached HEAD shouldn't block cockpit rendering.
		current = ""
	}
	needsRestack := s.computeNeedsRestackMap(ctx, g)
	logResult := logResultFromData(s.buildLogDataWithGraph(ctx, trunk, current, g, logOpts, needsRestack))
	items := checkoutItemsFromGraph(trunk, current, g, needsRestack)
	statuses := s.cachedPRStatusBranches(ctx, items)

	paused, err := s.PausedState(ctx)
	if err != nil {
		return nil, err
	}

	return &DashboardSnapshot{
		Trunk:         trunk,
		Current:       current,
		Log:           logResult,
		CheckoutItems: items,
		GitHubStatus:  statuses,
		Paused:        paused,
	}, nil
}

// PausedState returns the current paused stac-man op (a restack /
// sync / modify that hit a conflict) or nil when no op is paused.
// The cockpit's conflict resolver calls this directly when the user
// re-enters the screen after staging a fix, so it can refresh the
// conflict-paths list without re-reading the dashboard's full
// snapshot. Errors propagate so a stuck restack is never silently
// hidden behind an empty resolver.
func (s *Service) PausedState(ctx context.Context) (*PausedSnapshot, error) {
	info, ok, err := restack.New(s.G, s.Store).Paused(ctx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return info, nil
}
