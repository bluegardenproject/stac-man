package store

import "context"

// BranchMeta is the per-branch metadata stac-man tracks.
//
// All fields are optional individually but a branch is only considered
// "tracked" when Parent is set. ParentSHA records the parent's tip at
// the last clean restack so we can detect when the parent has moved
// and the branch needs restacking. PR is the GitHub PR number, set on
// first successful submit.
type BranchMeta struct {
	Parent    string
	ParentSHA string
	PR        int
}

// IsZero reports whether the metadata has no information at all.
// A branch with IsZero() == true is considered untracked.
func (b BranchMeta) IsZero() bool {
	return b.Parent == "" && b.ParentSHA == "" && b.PR == 0
}

// RepoMeta is the repo-level metadata stored alongside per-branch
// metadata.
type RepoMeta struct {
	// Trunk is the trunk branch (typically main / master). Empty when
	// not yet detected.
	Trunk string
	// Version is the schema version. Bumped when persisted layout
	// changes so future migrations have an anchor.
	Version int
}

// Store persists stac-man's metadata. The interface is narrow on
// purpose so the production implementation (git config) and the test
// fake (in-memory map) stay symmetric.
//
// Implementations must be safe to call concurrently from a single
// process; they are not expected to be safe across processes (git
// config writes are atomic per-key, which is enough for our flows).
type Store interface {
	// GetBranch returns the metadata for branch. If no metadata exists
	// it returns a zero-value BranchMeta with ok=false.
	GetBranch(ctx context.Context, branch string) (BranchMeta, bool, error)
	// SetBranch writes the metadata for branch. Implementations should
	// treat zero-valued fields as "leave untouched" only when the
	// existing key is absent; otherwise they must overwrite to keep
	// the on-disk representation in sync with what callers passed.
	SetBranch(ctx context.Context, branch string, meta BranchMeta) error
	// UnsetBranch removes all metadata for branch. Idempotent: removing
	// a branch that was never tracked must not error.
	UnsetBranch(ctx context.Context, branch string) error
	// ListTrackedBranches returns the names of every branch that has
	// at least a Parent set. Order is unspecified.
	ListTrackedBranches(ctx context.Context) ([]string, error)

	// GetRepo returns the repo-level metadata.
	GetRepo(ctx context.Context) (RepoMeta, error)
	// SetRepo writes the repo-level metadata.
	SetRepo(ctx context.Context, meta RepoMeta) error
}
