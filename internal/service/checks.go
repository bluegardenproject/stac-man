package service

import (
	"context"

	"github.com/philipptpunkt/stac-man/internal/gh"
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
			out[branch] = cached
			continue
		}
		status, err := client.PRStatusForNumber(ctx, pr.Number)
		if err != nil {
			continue
		}
		cache.Put(pr.Number, status)
		dirty = true
		out[branch] = status
	}
	if dirty {
		_ = cache.Save(gitDir)
	}
	return out
}
