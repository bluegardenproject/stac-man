// Package gitconfig is the production Store implementation. It
// persists stac-man metadata as `git config --local` keys so the data
// travels with the repo and survives clones / repacks transparently.
//
// Per-branch keys live under:
//
//	branch.<name>.stac-man-parent       # parent branch name
//	branch.<name>.stac-man-parent-sha   # parent tip SHA at last restack
//	branch.<name>.stac-man-pr           # GitHub PR number
//
// Repo-level keys live under:
//
//	stac-man.trunk
//	stac-man.version
package gitconfig

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/store"
)

// Schema version. Bump only when the on-disk layout changes; we read
// stac-man.version on first access and refuse to operate on newer
// versions than we understand.
const SchemaVersion = 1

// Key suffixes — exported so other layers can reference them by name.
const (
	keyParent    = "stac-man-parent"
	keyParentSHA = "stac-man-parent-sha"
	keyPR        = "stac-man-pr"

	repoKeyTrunk   = "stac-man.trunk"
	repoKeyVersion = "stac-man.version"
)

// configClient is the small slice of *git.Client gitconfig depends on.
// Splitting it out lets unit tests inject a fake without dragging the
// whole git package along.
type configClient interface {
	ConfigGet(ctx context.Context, key string) (string, bool, error)
	ConfigSet(ctx context.Context, key, value string) error
	ConfigUnset(ctx context.Context, key string) error
	ConfigList(ctx context.Context, prefix string) (map[string]string, error)
}

// Store implements store.Store on top of git config.
type Store struct {
	g configClient
}

// New constructs a Store backed by the given git client.
func New(g *git.Client) *Store {
	return &Store{g: g}
}

// NewWithClient is the seam used by unit tests.
func NewWithClient(g configClient) *Store {
	return &Store{g: g}
}

func branchKey(branch, suffix string) string {
	return "branch." + branch + "." + suffix
}

// GetBranch reads the three per-branch keys and assembles a BranchMeta.
// ok is true if at least one key was set.
func (s *Store) GetBranch(ctx context.Context, branch string) (store.BranchMeta, bool, error) {
	var meta store.BranchMeta
	any := false

	if v, ok, err := s.g.ConfigGet(ctx, branchKey(branch, keyParent)); err != nil {
		return meta, false, err
	} else if ok {
		meta.Parent = v
		any = true
	}
	if v, ok, err := s.g.ConfigGet(ctx, branchKey(branch, keyParentSHA)); err != nil {
		return meta, false, err
	} else if ok {
		meta.ParentSHA = v
		any = true
	}
	if v, ok, err := s.g.ConfigGet(ctx, branchKey(branch, keyPR)); err != nil {
		return meta, false, err
	} else if ok {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return meta, false, fmt.Errorf("parsing %s: %w", branchKey(branch, keyPR), perr)
		}
		meta.PR = n
		any = true
	}

	return meta, any, nil
}

// SetBranch writes whichever of the three keys are non-zero in meta
// and clears the rest. This makes SetBranch idempotent: the on-disk
// state always exactly reflects the BranchMeta passed in.
func (s *Store) SetBranch(ctx context.Context, branch string, meta store.BranchMeta) error {
	if err := s.writeOrUnset(ctx, branchKey(branch, keyParent), meta.Parent); err != nil {
		return err
	}
	if err := s.writeOrUnset(ctx, branchKey(branch, keyParentSHA), meta.ParentSHA); err != nil {
		return err
	}
	prVal := ""
	if meta.PR > 0 {
		prVal = strconv.Itoa(meta.PR)
	}
	return s.writeOrUnset(ctx, branchKey(branch, keyPR), prVal)
}

func (s *Store) writeOrUnset(ctx context.Context, key, value string) error {
	if value == "" {
		return s.g.ConfigUnset(ctx, key)
	}
	return s.g.ConfigSet(ctx, key, value)
}

// UnsetBranch clears all three per-branch keys.
func (s *Store) UnsetBranch(ctx context.Context, branch string) error {
	for _, suffix := range []string{keyParent, keyParentSHA, keyPR} {
		if err := s.g.ConfigUnset(ctx, branchKey(branch, suffix)); err != nil {
			return err
		}
	}
	return nil
}

// ListTrackedBranches scans the local config for `branch.*.stac-man-parent`
// entries and returns the unique branch names.
func (s *Store) ListTrackedBranches(ctx context.Context) ([]string, error) {
	all, err := s.g.ConfigList(ctx, "branch.")
	if err != nil {
		return nil, err
	}
	suffix := "." + keyParent
	out := make([]string, 0, len(all))
	for k := range all {
		if !strings.HasSuffix(k, suffix) {
			continue
		}
		// k = "branch.<name>.stac-man-parent" — strip prefix + suffix.
		name := strings.TrimSuffix(strings.TrimPrefix(k, "branch."), suffix)
		if name != "" {
			out = append(out, name)
		}
	}
	return out, nil
}

// GetRepo reads stac-man.trunk and stac-man.version.
func (s *Store) GetRepo(ctx context.Context) (store.RepoMeta, error) {
	var meta store.RepoMeta
	if v, ok, err := s.g.ConfigGet(ctx, repoKeyTrunk); err != nil {
		return meta, err
	} else if ok {
		meta.Trunk = v
	}
	if v, ok, err := s.g.ConfigGet(ctx, repoKeyVersion); err != nil {
		return meta, err
	} else if ok {
		n, perr := strconv.Atoi(v)
		if perr != nil {
			return meta, fmt.Errorf("parsing %s: %w", repoKeyVersion, perr)
		}
		meta.Version = n
	}
	return meta, nil
}

// SetRepo writes both repo-level keys.
func (s *Store) SetRepo(ctx context.Context, meta store.RepoMeta) error {
	if err := s.writeOrUnset(ctx, repoKeyTrunk, meta.Trunk); err != nil {
		return err
	}
	versionVal := ""
	if meta.Version > 0 {
		versionVal = strconv.Itoa(meta.Version)
	}
	return s.writeOrUnset(ctx, repoKeyVersion, versionVal)
}
