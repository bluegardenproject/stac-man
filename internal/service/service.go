// Package service is the orchestration layer between the cobra
// commands in cmd/ and the lower-level packages (git, gh, store,
// stack). Every user-facing operation lives here as a method, so a
// future TUI (v2) can drive the same logic without a second
// implementation.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/store"
	"github.com/philipptpunkt/stac-man/internal/store/gitconfig"
)

// Service wires the lower layers together. It is safe to construct
// once per command invocation; nothing here caches across calls.
type Service struct {
	G     *git.Client
	Store store.Store
}

// New constructs a Service rooted at the given working directory.
// Pass an empty dir to use the current process CWD.
func New(dir string) *Service {
	g := git.New(dir)
	return &Service{G: g, Store: gitconfig.New(g)}
}

// EnsureRepo asserts we're inside a git working tree. The error message
// is intentionally friendly because this is the first failure mode any
// new user hits.
func (s *Service) EnsureRepo(ctx context.Context) error {
	ok, err := s.G.IsRepo(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("not inside a git repository — run `sm` from a repo's working tree")
	}
	return nil
}

// EnsureTrunk returns the configured trunk branch, detecting and
// persisting it on first call. Detection order:
//
//  1. existing stac-man.trunk config
//  2. origin/HEAD's symbolic target (e.g. refs/remotes/origin/main)
//  3. local "main" branch
//  4. local "master" branch
//
// Returns an error if none of the above resolves.
func (s *Service) EnsureTrunk(ctx context.Context) (string, error) {
	repo, err := s.Store.GetRepo(ctx)
	if err != nil {
		return "", err
	}
	if repo.Trunk != "" {
		return repo.Trunk, nil
	}

	trunk, err := s.detectTrunk(ctx)
	if err != nil {
		return "", err
	}
	repo.Trunk = trunk
	if repo.Version == 0 {
		repo.Version = gitconfig.SchemaVersion
	}
	if err := s.Store.SetRepo(ctx, repo); err != nil {
		return "", fmt.Errorf("persisting detected trunk: %w", err)
	}
	return trunk, nil
}

func (s *Service) detectTrunk(ctx context.Context) (string, error) {
	// origin/HEAD points at refs/remotes/origin/<default>; strip the
	// prefix to get the branch name.
	if target, ok, err := s.G.SymbolicRef(ctx, "refs/remotes/origin/HEAD"); err == nil && ok {
		const prefix = "refs/remotes/origin/"
		if strings.HasPrefix(target, prefix) {
			candidate := strings.TrimPrefix(target, prefix)
			if exists, _ := s.G.BranchExists(ctx, candidate); exists {
				return candidate, nil
			}
		}
	}
	for _, candidate := range []string{"main", "master"} {
		if exists, _ := s.G.BranchExists(ctx, candidate); exists {
			return candidate, nil
		}
	}
	return "", errors.New("could not detect trunk: set `git config stac-man.trunk <branch>` to override")
}
