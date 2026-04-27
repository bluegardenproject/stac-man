package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/git"
	"github.com/philipptpunkt/stac-man/internal/stack"
	"github.com/philipptpunkt/stac-man/internal/store"
)

// SplitMapping is one new branch carved out of the original.
type SplitMapping struct {
	// Name is the new branch name. Required, must be unique in the
	// mapping.
	Name string
	// Commits are the SHAs that go into this branch, in oldest-first
	// order. SHAs must come from the original branch's commit list
	// (i.e. commits unique to the branch vs its parent).
	Commits []string
}

// SplitOptions configures Split.
type SplitOptions struct {
	// Mappings, when non-empty, drives a deterministic split. When
	// empty, Split auto-generates one mapping per commit using the
	// commit subject as the branch name (slugified).
	Mappings []SplitMapping
}

// SplitResult summarizes the per-branch outcome of a split.
type SplitResult struct {
	Original string
	Created  []string
}

// Split decomposes the current branch into a chain of new branches,
// one per mapping. Each new branch's parent is the previous mapping
// (or the original parent for the first mapping). The original branch
// is untracked and deleted; any of its tracked children are
// reparented onto the topmost new branch.
func (s *Service) Split(ctx context.Context, opts SplitOptions) (SplitResult, error) {
	r := SplitResult{}
	if err := s.EnsureRepo(ctx); err != nil {
		return r, err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return r, err
	}
	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return r, err
	}
	if current == trunk {
		return r, errors.New("refusing to split trunk")
	}
	r.Original = current

	if clean, err := s.G.IsClean(ctx); err != nil {
		return r, err
	} else if !clean {
		return r, errors.New("working tree is dirty; commit or stash first")
	}

	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return r, err
	}
	cur, ok := g.Get(current)
	if !ok {
		return r, fmt.Errorf("branch %q is not tracked", current)
	}
	parent := cur.Parent
	if parent == "" {
		parent = trunk
	}

	commits, err := s.G.LogBetween(ctx, parent, current)
	if err != nil {
		return r, fmt.Errorf("listing commits unique to %s: %w", current, err)
	}
	if len(commits) < 2 {
		return r, fmt.Errorf("nothing to split: %s has %d commit(s) beyond %s", current, len(commits), parent)
	}

	mappings := opts.Mappings
	if len(mappings) == 0 {
		mappings = autoMappings(commits)
	}
	if err := validateMappings(mappings, commits); err != nil {
		return r, err
	}
	if err := s.ensureNewBranchesFree(ctx, mappings); err != nil {
		return r, err
	}

	originalChildren := g.ChildrenOf(current)

	touched := []string{current}
	for _, m := range mappings {
		touched = append(touched, m.Name)
	}
	for _, child := range originalChildren {
		touched = append(touched, child.Name)
	}
	s.recordHistory(ctx, "split", current, touched)

	// Build each new branch in turn. We work off of `parent` and walk
	// upward, so each step's start ref is the previous step's tip.
	createdNames := make([]string, 0, len(mappings))
	previousBranch := parent
	for _, m := range mappings {
		if err := s.G.SwitchCreate(ctx, m.Name, previousBranch); err != nil {
			return r, fmt.Errorf("creating %s on top of %s: %w", m.Name, previousBranch, err)
		}
		for _, sha := range m.Commits {
			if err := s.G.CherryPick(ctx, sha); err != nil {
				// Best-effort cleanup so the user isn't left in a
				// half-built split: abort any in-flight cherry-pick
				// and roll back created branches.
				_ = s.G.CherryPickAbort(ctx)
				s.cleanupSplit(ctx, current, createdNames, m.Name)
				return r, fmt.Errorf("cherry-picking %s onto %s: %w", short(sha), m.Name, err)
			}
		}
		previousBranchSHA, err := s.G.RevParse(ctx, previousBranch)
		if err != nil {
			return r, err
		}
		meta := store.BranchMeta{Parent: previousBranch, ParentSHA: previousBranchSHA}
		if err := s.Store.SetBranch(ctx, m.Name, meta); err != nil {
			return r, fmt.Errorf("recording metadata for %s: %w", m.Name, err)
		}
		createdNames = append(createdNames, m.Name)
		previousBranch = m.Name
	}

	// Reparent original children onto the topmost new branch.
	topName := createdNames[len(createdNames)-1]
	topSHA, err := s.G.RevParse(ctx, topName)
	if err != nil {
		return r, err
	}
	for _, child := range originalChildren {
		meta, _, err := s.Store.GetBranch(ctx, child.Name)
		if err != nil {
			return r, err
		}
		meta.Parent = topName
		meta.ParentSHA = topSHA
		if err := s.Store.SetBranch(ctx, child.Name, meta); err != nil {
			return r, err
		}
	}

	// Untrack and delete the original.
	if err := s.Store.UnsetBranch(ctx, current); err != nil {
		return r, fmt.Errorf("untracking %s: %w", current, err)
	}
	if err := s.G.DeleteBranch(ctx, current, true); err != nil {
		return r, fmt.Errorf("deleting %s: %w", current, err)
	}

	r.Created = createdNames
	return r, nil
}

// autoMappings generates one mapping per commit. The branch name is the
// slugified subject; if two subjects collide, a numeric suffix is added.
func autoMappings(commits []git.Commit) []SplitMapping {
	out := make([]SplitMapping, 0, len(commits))
	seen := map[string]int{}
	for _, c := range commits {
		base := slugify(c.Subject)
		if base == "" {
			base = "commit-" + short(c.SHA)
		}
		name := base
		if n, dup := seen[base]; dup {
			seen[base] = n + 1
			name = fmt.Sprintf("%s-%d", base, n+1)
		} else {
			seen[base] = 1
		}
		out = append(out, SplitMapping{Name: name, Commits: []string{c.SHA}})
	}
	return out
}

// validateMappings checks that mappings cover every commit exactly
// once and in topo (oldest-first) order, that no name is reused, and
// that names look like valid branch names.
func validateMappings(mappings []SplitMapping, commits []git.Commit) error {
	if len(mappings) == 0 {
		return errors.New("split requires at least one mapping")
	}
	if len(mappings) < 2 {
		return errors.New("split needs at least two mappings; for a single branch nothing changes")
	}
	indexBySHA := map[string]int{}
	for i, c := range commits {
		indexBySHA[c.SHA] = i
	}

	seenSHAs := map[string]bool{}
	seenNames := map[string]bool{}
	expected := 0
	for _, m := range mappings {
		if m.Name == "" {
			return errors.New("mapping is missing a name")
		}
		if !validBranchName(m.Name) {
			return fmt.Errorf("invalid branch name %q", m.Name)
		}
		if seenNames[m.Name] {
			return fmt.Errorf("duplicate branch name %q in mappings", m.Name)
		}
		seenNames[m.Name] = true
		if len(m.Commits) == 0 {
			return fmt.Errorf("mapping %q has no commits", m.Name)
		}
		for _, sha := range m.Commits {
			idx, known := indexBySHA[sha]
			if !known {
				return fmt.Errorf("commit %s is not part of the branch", short(sha))
			}
			if seenSHAs[sha] {
				return fmt.Errorf("commit %s appears in multiple mappings", short(sha))
			}
			seenSHAs[sha] = true
			if idx != expected {
				return fmt.Errorf("mapping %q is out of topological order at %s", m.Name, short(sha))
			}
			expected++
		}
	}
	if expected != len(commits) {
		return fmt.Errorf("mappings cover %d commits but branch has %d", expected, len(commits))
	}
	return nil
}

// ensureNewBranchesFree errors if any mapping name already exists as a
// local branch — we don't want split to clobber.
func (s *Service) ensureNewBranchesFree(ctx context.Context, mappings []SplitMapping) error {
	for _, m := range mappings {
		exists, err := s.G.BranchExists(ctx, m.Name)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("branch %q already exists; pick a different name", m.Name)
		}
	}
	return nil
}

// cleanupSplit best-effort rolls back partially-created branches when
// a split aborts mid-way. Leaves the user back on the original branch.
func (s *Service) cleanupSplit(ctx context.Context, original string, created []string, currentlyOn string) {
	// Hop off the branch we're about to delete.
	_ = s.G.Checkout(ctx, original)
	// Drop any created branches in reverse order.
	if currentlyOn != "" {
		_ = s.Store.UnsetBranch(ctx, currentlyOn)
		_ = s.G.DeleteBranch(ctx, currentlyOn, true)
	}
	for i := len(created) - 1; i >= 0; i-- {
		_ = s.Store.UnsetBranch(ctx, created[i])
		_ = s.G.DeleteBranch(ctx, created[i], true)
	}
}

// ParseRanges expands a comma-separated list of 1-based commit ranges
// (e.g. "1-2,4,6-8") into a flat slice of indices into the commit list.
// Used by the cmd layer to build mappings from --commits.
func ParseRanges(spec string, total int) ([]int, error) {
	out := []int{}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if dash := strings.Index(part, "-"); dash > 0 {
			a, errA := strconv.Atoi(strings.TrimSpace(part[:dash]))
			b, errB := strconv.Atoi(strings.TrimSpace(part[dash+1:]))
			if errA != nil || errB != nil || a < 1 || b < a || b > total {
				return nil, fmt.Errorf("invalid range %q (commits go 1..%d)", part, total)
			}
			for i := a; i <= b; i++ {
				out = append(out, i-1)
			}
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > total {
			return nil, fmt.Errorf("invalid index %q (commits go 1..%d)", part, total)
		}
		out = append(out, n-1)
	}
	return out, nil
}

var slugRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	return s
}

var branchRE = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func validBranchName(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") || strings.Contains(s, "..") {
		return false
	}
	return branchRE.MatchString(s)
}

// CommitsForCurrent returns the commits unique to current vs. its
// recorded parent. Exposed so the cmd layer can render a numbered
// preview before parsing --commits ranges.
func (s *Service) CommitsForCurrent(ctx context.Context) ([]git.Commit, string, error) {
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, "", err
	}
	trunk, err := s.EnsureTrunk(ctx)
	if err != nil {
		return nil, "", err
	}
	current, err := s.G.CurrentBranch(ctx)
	if err != nil {
		return nil, "", err
	}
	g, err := stack.Load(ctx, s.Store)
	if err != nil {
		return nil, "", err
	}
	parent := trunk
	if cur, ok := g.Get(current); ok && cur.Parent != "" {
		parent = cur.Parent
	}
	commits, err := s.G.LogBetween(ctx, parent, current)
	return commits, current, err
}
