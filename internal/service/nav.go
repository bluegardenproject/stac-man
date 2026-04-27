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

// CheckoutItem is one row in the tree-shaped data set returned by
// [Service.CheckoutTree]. It carries enough information for an
// interactive picker (or a plain text fallback) to render the stack
// graph as a selectable list. Callers translate this into UI by
// reading the depth + ancestor flags to draw connectors and the
// status flags (IsCurrent, NeedsRestack, …) to colour the row.
type CheckoutItem struct {
	// Branch is the branch name. For the trunk row this is the
	// trunk's name (e.g. "main").
	Branch string
	// Depth is the distance from the trunk: 0 for the trunk row, 1
	// for branches whose parent is the trunk, and so on.
	Depth int
	// IsTrunk is true only for the single trunk row at the top of
	// the list.
	IsTrunk bool
	// IsCurrent is true for the row matching the branch HEAD is
	// currently on. Always false on detached HEAD.
	IsCurrent bool
	// NeedsRestack is true when the branch's recorded ParentSHA no
	// longer matches its parent's tip. Only set for non-trunk rows.
	NeedsRestack bool
	// PR is the GitHub PR number recorded in the store, or 0 when
	// no PR has been linked yet.
	PR int
	// AncestorIsLast has one entry per ancestor between this row
	// and the trunk (excluding this row's immediate parent). Each
	// boolean records whether that ancestor was the last child of
	// its parent at render time. Renderers use it to decide whether
	// to draw a vertical pipe ("│  ") or empty padding ("   ") at
	// each indentation level. Always nil for the trunk row.
	AncestorIsLast []bool
	// IsLastChild is true when this row is the alphabetically last
	// child of its parent. Renderers use it to choose between the
	// "├─ " and "└─ " connectors. Ignored when IsTrunk is true.
	IsLastChild bool
}

// CheckoutTree returns the trunk plus every tracked branch as a
// depth-first, alphabetically-ordered list of [CheckoutItem]s. The
// shape mirrors what `sm log` renders, so an interactive picker can
// turn the slice straight into a selectable stack tree without
// re-implementing graph traversal.
func (s *Service) CheckoutTree(ctx context.Context) (current string, items []CheckoutItem, err error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return "", nil, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return "", nil, err
	}
	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return "", nil, err
	}
	current, _ = s.G.CurrentBranch(ctx)

	items = append(items, CheckoutItem{
		Branch:    trunk,
		Depth:     0,
		IsTrunk:   true,
		IsCurrent: trunk == current,
	})

	var walk func(parent string, depth int, ancestors []bool)
	walk = func(parent string, depth int, ancestors []bool) {
		children := g.ChildrenOf(parent)
		for i, child := range children {
			isLast := i == len(children)-1
			// Only ancestors *above* this row's immediate parent
			// produce indentation pipes; the row's own connector is
			// chosen from IsLastChild. So we hand the child the
			// current ancestors slice without appending isLast.
			ancestorsCopy := append([]bool(nil), ancestors...)
			items = append(items, CheckoutItem{
				Branch:         child.Name,
				Depth:          depth,
				IsCurrent:      child.Name == current,
				NeedsRestack:   s.needsRestack(child),
				PR:             child.PR,
				AncestorIsLast: ancestorsCopy,
				IsLastChild:    isLast,
			})
			walk(child.Name, depth+1, append(ancestors, isLast))
		}
	}
	walk(trunk, 1, nil)

	return current, items, nil
}

// CheckoutChoices returns trunk + every tracked branch as a flat,
// sorted list with the current branch pinned at the front. The
// interactive picker uses [Service.CheckoutTree] instead; this
// flatter view is kept for future fzf-style integrations
// (see ROADMAP "Fuzzy sm checkout").
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
//     preferAlpha=true, in which case alphabetically first)
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
