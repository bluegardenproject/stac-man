package stack

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/bluegardenproject/stac-man/internal/store"
)

// Branch is a single node in the stack graph.
type Branch struct {
	Name      string
	Parent    string // empty when this branch is a root (sits directly on trunk)
	ParentSHA string
	PR        int
}

// Graph is a snapshot of every tracked branch in the repo plus the
// trunk it sits on. It's a pure value type — no I/O after Load — so
// commands can call ChildrenOf / Descendants / Roots cheaply many
// times.
type Graph struct {
	Trunk    string
	branches map[string]Branch
}

// ErrCycle indicates the persisted parent graph contains a cycle.
// Validate returns this so `sm doctor` can surface it precisely.
var ErrCycle = errors.New("stack graph contains a cycle")

// Load reads every tracked branch from the store and assembles a
// Graph. Trunk is taken from RepoMeta; if it's empty Load returns the
// graph anyway so callers like `sm doctor` can report the missing
// trunk explicitly.
func Load(ctx context.Context, s store.Store) (*Graph, error) {
	repo, err := s.GetRepo(ctx)
	if err != nil {
		return nil, err
	}
	names, err := s.ListTrackedBranches(ctx)
	if err != nil {
		return nil, err
	}

	g := &Graph{
		Trunk:    repo.Trunk,
		branches: make(map[string]Branch, len(names)),
	}
	for _, name := range names {
		meta, ok, err := s.GetBranch(ctx, name)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		g.branches[name] = Branch{
			Name:      name,
			Parent:    meta.Parent,
			ParentSHA: meta.ParentSHA,
			PR:        meta.PR,
		}
	}
	return g, nil
}

// Branches returns every tracked branch sorted by name. The slice is a
// fresh copy — callers can mutate it freely.
func (g *Graph) Branches() []Branch {
	out := make([]Branch, 0, len(g.branches))
	for _, b := range g.branches {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns the metadata for branch, or ok=false if not tracked.
func (g *Graph) Get(name string) (Branch, bool) {
	b, ok := g.branches[name]
	return b, ok
}

// IsTracked is a convenience over Get.
func (g *Graph) IsTracked(name string) bool {
	_, ok := g.branches[name]
	return ok
}

// Roots returns every tracked branch whose parent is the trunk.
// Sorted by name for deterministic output.
func (g *Graph) Roots() []Branch {
	out := []Branch{}
	for _, b := range g.branches {
		if b.Parent == g.Trunk {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ChildrenOf returns the tracked branches whose parent is name.
// Includes branches whose parent is the trunk when name == g.Trunk.
func (g *Graph) ChildrenOf(name string) []Branch {
	out := []Branch{}
	for _, b := range g.branches {
		if b.Parent == name {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Ancestors returns the chain of parent branches from name up to (and
// excluding) the trunk. The first element is name's immediate parent.
// Returns an empty slice if name has no recorded parent.
func (g *Graph) Ancestors(name string) []Branch {
	out := []Branch{}
	cur, ok := g.branches[name]
	if !ok {
		return out
	}
	seen := map[string]bool{name: true}
	for cur.Parent != "" && cur.Parent != g.Trunk {
		next, ok := g.branches[cur.Parent]
		if !ok {
			break
		}
		if seen[next.Name] {
			break // cycle guard; Validate will report it
		}
		seen[next.Name] = true
		out = append(out, next)
		cur = next
	}
	return out
}

// Descendants returns every tracked branch reachable downward from
// name, in depth-first preorder. Sibling order at each level is by
// name for determinism. Branch `name` itself is NOT included.
func (g *Graph) Descendants(name string) []Branch {
	visited := map[string]bool{}
	var out []Branch
	var walk func(parent string)
	walk = func(parent string) {
		for _, child := range g.ChildrenOf(parent) {
			if visited[child.Name] {
				continue
			}
			visited[child.Name] = true
			out = append(out, child)
			walk(child.Name)
		}
	}
	walk(name)
	return out
}

// TopoOrderFrom returns name plus all its descendants in a parents-
// before-children ordering. Useful for the restack walk: replaying
// rebases in this order guarantees each branch's parent is already at
// its post-restack tip.
func (g *Graph) TopoOrderFrom(name string) []Branch {
	out := []Branch{}
	if b, ok := g.branches[name]; ok {
		out = append(out, b)
	}
	out = append(out, g.Descendants(name)...)
	return out
}

// Validate returns a list of human-readable errors describing problems
// with the graph: cycles, parents that don't exist locally, branches
// pointing at nothing. An empty result means the graph is healthy.
func (g *Graph) Validate(localBranches []string) []error {
	local := make(map[string]bool, len(localBranches))
	for _, b := range localBranches {
		local[b] = true
	}

	var errs []error

	// Check every tracked branch exists locally.
	for name := range g.branches {
		if !local[name] {
			errs = append(errs, fmt.Errorf("tracked branch %q has no local ref", name))
		}
	}

	// Cycle / orphan detection via repeated walks. We use a small DFS
	// with a "currently visiting" set so we catch cycles regardless of
	// where Load happened to start iterating.
	color := map[string]int{} // 0=unseen, 1=visiting, 2=done
	var visit func(name string) error
	visit = func(name string) error {
		if color[name] == 2 {
			return nil
		}
		if color[name] == 1 {
			return fmt.Errorf("%w: %q", ErrCycle, name)
		}
		color[name] = 1
		b, ok := g.branches[name]
		if !ok {
			color[name] = 2
			return nil
		}
		if b.Parent != "" && b.Parent != g.Trunk {
			if _, parentTracked := g.branches[b.Parent]; !parentTracked {
				errs = append(errs, fmt.Errorf("branch %q has untracked parent %q", name, b.Parent))
			} else if err := visit(b.Parent); err != nil {
				return err
			}
		}
		color[name] = 2
		return nil
	}

	// Make iteration deterministic by sorting names.
	names := make([]string, 0, len(g.branches))
	for n := range g.branches {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name); err != nil {
			errs = append(errs, err)
			break // a cycle taints the whole graph; one report is enough
		}
	}

	return errs
}
