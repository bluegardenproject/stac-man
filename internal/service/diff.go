package service

import (
	"context"
	"fmt"
)

// CommitDiff returns the unified diff of a single commit (matching
// `git show <sha>` semantics: the commit metadata followed by the
// patch). Used by the cockpit's diff viewer to render per-commit
// content as the user moves through a branch's commit list.
//
// Errors from the git wrapper are surfaced verbatim so the viewer
// can show "git show <sha> failed: ..." rather than collapsing
// every failure into a generic empty pane.
func (s *Service) CommitDiff(ctx context.Context, sha string) (string, error) {
	if sha == "" {
		return "", fmt.Errorf("CommitDiff: empty sha")
	}
	if err := s.EnsureRepo(ctx); err != nil {
		return "", err
	}
	return s.G.Show(ctx, sha)
}
