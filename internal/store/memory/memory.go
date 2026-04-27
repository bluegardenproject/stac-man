// Package memory provides an in-memory Store for unit tests. It has
// no production callers — production uses store/gitconfig.
package memory

import (
	"context"
	"sync"

	"github.com/philipptpunkt/stac-man/internal/store"
)

// Store is an in-memory implementation of store.Store.
type Store struct {
	mu       sync.Mutex
	branches map[string]store.BranchMeta
	repo     store.RepoMeta
}

// New returns an empty memory store.
func New() *Store {
	return &Store{branches: map[string]store.BranchMeta{}}
}

func (s *Store) GetBranch(_ context.Context, branch string) (store.BranchMeta, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, ok := s.branches[branch]
	return m, ok, nil
}

func (s *Store) SetBranch(_ context.Context, branch string, meta store.BranchMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.branches[branch] = meta
	return nil
}

func (s *Store) UnsetBranch(_ context.Context, branch string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.branches, branch)
	return nil
}

func (s *Store) ListTrackedBranches(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.branches))
	for name, meta := range s.branches {
		if meta.Parent != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

func (s *Store) GetRepo(_ context.Context) (store.RepoMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo, nil
}

func (s *Store) SetRepo(_ context.Context, meta store.RepoMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.repo = meta
	return nil
}
