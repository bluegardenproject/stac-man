package service

import (
	"context"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/ui/progress"
)

// statusFetchOptions controls which subset of PRStatus the caller
// actually needs. Both `sm log` and `sm doctor` share the same gh
// round-trip, but they may opt out of one channel via flags. We
// still issue the fetch when EITHER channel is wanted because the
// gh response carries both fields together — splitting the call
// would cost an extra round-trip per PR for no benefit.
type statusFetchOptions struct {
	wantChecks bool
	wantMerge  bool
	// progress is optional; nil collapses to a discard reporter so
	// `sm doctor` (which doesn't surface progress today) inherits a
	// silent default while `sm log` can pass a real spinner.
	progress progress.Reporter
}

// fetchPRStatuses returns one gh.PRStatus per branch in prMap, using
// the on-disk cache to skip recently-fetched entries. Drafts, merged
// PRs, and closed PRs are NOT skipped at this layer — the renderer
// decides per-feature whether to surface a glyph (the roadmap calls
// out that the merge glyph hides on drafts but the CI dot can still
// be useful there).
//
// This is a best-effort helper: any failure (no gh, no auth, parse
// errors) collapses to "no status for that PR" rather than failing
// the whole command. Callers fall back to plain rendering when the
// returned map is empty.
func (s *Service) fetchPRStatuses(ctx context.Context, prMap map[string]gh.PR, opts statusFetchOptions) map[string]gh.PRStatus {
	out := map[string]gh.PRStatus{}
	if !opts.wantChecks && !opts.wantMerge {
		return out
	}
	if len(prMap) == 0 {
		return out
	}
	if err := gh.PreflightCheck(ctx); err != nil {
		return out
	}

	prog := opts.progress
	if prog == nil {
		prog = progress.Discard()
	}

	gitDir, _ := s.G.GitDir(ctx)
	cache := gh.LoadChecksCache(gitDir)
	client := gh.New("")

	dirty := false
	for branch, pr := range prMap {
		if pr.Number == 0 {
			continue
		}
		if pr.State == gh.PRStateMerged || pr.State == gh.PRStateClosed {
			// Closed and merged PRs never flip back to a state that
			// would change the glyph. Skip the round-trip entirely.
			continue
		}
		if cached, ok := cache.Get(pr.Number); ok {
			// Cached hits don't get a progress line — they return in
			// microseconds and the user shouldn't see any animation
			// for free reads.
			out[branch] = cached
			continue
		}
		prog.Start(fmt.Sprintf("fetching status for PR #%d", pr.Number))
		status, err := client.PRStatusForNumber(ctx, pr.Number)
		if err != nil {
			prog.Fail(fmt.Sprintf("fetching status for PR #%d", pr.Number))
			continue
		}
		prog.Done(fmt.Sprintf("fetched status for PR #%d", pr.Number))
		cache.Put(pr.Number, status)
		dirty = true
		out[branch] = status
	}
	if dirty {
		_ = cache.Save(gitDir)
	}
	return out
}
