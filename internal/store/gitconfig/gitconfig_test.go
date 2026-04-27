package gitconfig

import (
	"context"
	"sort"
	"testing"

	"github.com/philipptpunkt/stac-man/internal/store"
)

// fakeConfig is an in-memory configClient backed by a map. It mimics
// `git config --local` semantics: missing keys return ok=false rather
// than erroring, and unset on a missing key is a no-op.
type fakeConfig struct {
	values map[string]string
}

func newFake() *fakeConfig {
	return &fakeConfig{values: map[string]string{}}
}

func (f *fakeConfig) ConfigGet(_ context.Context, key string) (string, bool, error) {
	v, ok := f.values[key]
	return v, ok, nil
}

func (f *fakeConfig) ConfigSet(_ context.Context, key, value string) error {
	f.values[key] = value
	return nil
}

func (f *fakeConfig) ConfigUnset(_ context.Context, key string) error {
	delete(f.values, key)
	return nil
}

func (f *fakeConfig) ConfigList(_ context.Context, prefix string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range f.values {
		if prefix == "" || hasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

func TestSetGetRoundTrip(t *testing.T) {
	s := NewWithClient(newFake())
	ctx := context.Background()

	in := store.BranchMeta{Parent: "main", ParentSHA: "abc123", PR: 42}
	if err := s.SetBranch(ctx, "feat-a", in); err != nil {
		t.Fatalf("SetBranch: %v", err)
	}
	got, ok, err := s.GetBranch(ctx, "feat-a")
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != in {
		t.Fatalf("got %+v, want %+v", got, in)
	}
}

func TestSetClearsOmittedFields(t *testing.T) {
	fc := newFake()
	s := NewWithClient(fc)
	ctx := context.Background()

	// Seed with a full record.
	if err := s.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", ParentSHA: "abc", PR: 7}); err != nil {
		t.Fatalf("SetBranch seed: %v", err)
	}
	// Now write only Parent. ParentSHA and PR should be cleared.
	if err := s.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main"}); err != nil {
		t.Fatalf("SetBranch update: %v", err)
	}
	if _, exists := fc.values["branch.feat-a.stac-man-parent-sha"]; exists {
		t.Errorf("expected parent-sha key to be cleared")
	}
	if _, exists := fc.values["branch.feat-a.stac-man-pr"]; exists {
		t.Errorf("expected pr key to be cleared")
	}
}

func TestUnsetBranch(t *testing.T) {
	fc := newFake()
	s := NewWithClient(fc)
	ctx := context.Background()

	if err := s.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", PR: 9}); err != nil {
		t.Fatalf("SetBranch: %v", err)
	}
	if err := s.UnsetBranch(ctx, "feat-a"); err != nil {
		t.Fatalf("UnsetBranch: %v", err)
	}
	if len(fc.values) != 0 {
		t.Fatalf("expected empty config, got %v", fc.values)
	}
	// Unsetting again must be a no-op, not an error.
	if err := s.UnsetBranch(ctx, "feat-a"); err != nil {
		t.Fatalf("UnsetBranch idempotent: %v", err)
	}
}

func TestListTrackedBranches(t *testing.T) {
	fc := newFake()
	s := NewWithClient(fc)
	ctx := context.Background()

	for branch, parent := range map[string]string{
		"feat-a": "main",
		"feat-b": "feat-a",
		"feat-c": "feat-a",
	} {
		if err := s.SetBranch(ctx, branch, store.BranchMeta{Parent: parent}); err != nil {
			t.Fatalf("SetBranch %s: %v", branch, err)
		}
	}
	// Add an unrelated branch.* key that should NOT be picked up.
	fc.values["branch.unrelated.remote"] = "origin"

	got, err := s.ListTrackedBranches(ctx)
	if err != nil {
		t.Fatalf("ListTrackedBranches: %v", err)
	}
	sort.Strings(got)
	want := []string{"feat-a", "feat-b", "feat-c"}
	if !equalSlice(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRepoRoundTrip(t *testing.T) {
	s := NewWithClient(newFake())
	ctx := context.Background()
	in := store.RepoMeta{Trunk: "main", Version: SchemaVersion}
	if err := s.SetRepo(ctx, in); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	got, err := s.GetRepo(ctx)
	if err != nil {
		t.Fatalf("GetRepo: %v", err)
	}
	if got != in {
		t.Fatalf("got %+v, want %+v", got, in)
	}
}

func TestGetBranchUnknownReturnsZero(t *testing.T) {
	s := NewWithClient(newFake())
	got, ok, err := s.GetBranch(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetBranch: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing branch")
	}
	if !got.IsZero() {
		t.Fatalf("expected zero meta, got %+v", got)
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

