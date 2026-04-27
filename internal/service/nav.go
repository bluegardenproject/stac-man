package service

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/philipptpunkt/stac-man/internal/stack"
)

// Checkout switches HEAD to the named branch. If branch is empty,
// returns the list of tracked branches plus the trunk so callers can
// render an interactive picker.
func (s *Service) Checkout(ctx context.Context, branch string) error {
	if err := s.EnsureRepo(ctx); err != nil {
		return err
	}
	if branch == "" {
		return errors.New("branch name required")
	}
	return s.G.Checkout(ctx, branch)
}

// CheckoutChoices returns trunk + every tracked branch, sorted, with
// the current branch first. Used by the interactive picker.
func (s *Service) CheckoutChoices(ctx context.Context) (current string, choices []string, err error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", nil, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return "", nil, err
	}
	tracked, err := s.Store.ListTrackedBranches(ctx)
	if err != nil {
		return "", nil, err
	}
	current, _ = s.G.CurrentBranch(ctx)

	all := append([]string{trunk}, tracked...)
	sort.Strings(all)

	// Move current to the front for affordance.
	if current != "" {
		out := []string{current}
		for _, b := range all {
			if b != current {
				out = append(out, b)
			}
		}
		all = out
	}
	return current, all, nil
}

// Direction describes which way a navigation command moves through
// the stack graph.
type Direction int

const (
	DirUp Direction = iota
	DirDown
	DirTop
	DirBottom
)

// Navigate moves HEAD relative to the current branch's position in
// the stack. Returns the branch checked out.
//
//   - DirUp:     first child of current (errors if multiple unless
//                preferAlpha=true, in which case alphabetically first)
//   - DirDown:   parent of current (errors if current is on trunk)
//   - DirTop:    walk children to a leaf (alpha at forks)
//   - DirBottom: walk parents until just above the trunk
func (s *Service) Navigate(ctx context.Context, dir Direction, preferAlpha bool) (string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return "", err
	}
	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return "", err
	}
	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return "", err
	}

	target, err := resolveDirection(g, trunk, current, dir, preferAlpha)
	if err != nil {
		return "", err
	}
	if target == current {
		return current, nil
	}
	if err := s.G.Checkout(ctx, target); err != nil {
		return "", err
	}
	return target, nil
}

func resolveDirection(g *stack.Graph, trunk, current string, dir Direction, preferAlpha bool) (string, error) {
	switch dir {
	case DirDown:
		if current == trunk {
			return "", errors.New("already on trunk")
		}
		b, ok := g.Get(current)
		if !ok {
			return "", fmt.Errorf("current branch %q is not tracked", current)
		}
		if b.Parent == "" {
			return trunk, nil
		}
		return b.Parent, nil

	case DirUp:
		children := g.ChildrenOf(current)
		if len(children) == 0 {
			return "", errors.New("no children to move up to")
		}
		if len(children) > 1 && !preferAlpha {
			names := make([]string, len(children))
			for i, c := range children {
				names[i] = c.Name
			}
			return "", fmt.Errorf("multiple children: %v — pass --first to take the alphabetically first one", names)
		}
		// children is already sorted alpha by ChildrenOf
		return children[0].Name, nil

	case DirBottom:
		// Walk parents until we reach a branch whose parent is the trunk.
		cur := current
		seen := map[string]bool{cur: true}
		for {
			b, ok := g.Get(cur)
			if !ok {
				if cur == trunk {
					return "", errors.New("already on trunk")
				}
				return cur, nil
			}
			if b.Parent == "" || b.Parent == trunk {
				return cur, nil
			}
			if seen[b.Parent] {
				return "", errors.New("cycle detected while walking down")
			}
			seen[b.Parent] = true
			cur = b.Parent
		}

	case DirTop:
		// Walk to a leaf, choosing alpha-first child at each fork.
		cur := current
		seen := map[string]bool{cur: true}
		for {
			children := g.ChildrenOf(cur)
			if len(children) == 0 {
				return cur, nil
			}
			if len(children) > 1 && !preferAlpha {
				names := make([]string, len(children))
				for i, c := range children {
					names[i] = c.Name
				}
				return "", fmt.Errorf("fork at %q with children %v — pass --first to take the alphabetically first one", cur, names)
			}
			next := children[0].Name
			if seen[next] {
				return "", errors.New("cycle detected while walking up")
			}
			seen[next] = true
			cur = next
		}
	}
	return "", fmt.Errorf("unknown direction %d", dir)
}
