package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/cache"
	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/git"
	"github.com/bluegardenproject/stac-man/internal/stack"
	"github.com/bluegardenproject/stac-man/internal/store"
	"github.com/bluegardenproject/stac-man/internal/store/memory"
)

type fakeStatusClient struct {
	byPR map[int]gh.PRStatus
	errs map[int]error
}

func (f fakeStatusClient) PRStatusForNumber(_ context.Context, number int) (gh.PRStatus, error) {
	if err, ok := f.errs[number]; ok {
		return gh.PRStatus{}, err
	}
	return f.byPR[number], nil
}

func TestStatusFetchesLiveStatusAndPersistsCache(t *testing.T) {
	ctx := context.Background()
	gitDir := filepath.Join(t.TempDir(), ".git")
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", PR: 10}); err != nil {
		t.Fatalf("SetBranch feat-a: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-b", store.BranchMeta{Parent: "feat-a", PR: 11}); err != nil {
		t.Fatalf("SetBranch feat-b: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-local", store.BranchMeta{Parent: "main"}); err != nil {
		t.Fatalf("SetBranch feat-local: %v", err)
	}

	oldPreflight := statusPreflightCheck
	oldClient := newStatusClient
	defer func() {
		statusPreflightCheck = oldPreflight
		newStatusClient = oldClient
	}()
	statusPreflightCheck = func(context.Context) error { return nil }
	newStatusClient = func() prStatusClient {
		return fakeStatusClient{byPR: map[int]gh.PRStatus{
			10: {Number: 10, Checks: gh.ChecksPass, Mergeable: gh.MergeMergeable, State: gh.PRStateOpen},
			11: {Number: 11, Checks: gh.ChecksFail, Mergeable: gh.MergeConflicting, State: gh.PRStateOpen, IsDraft: true},
		}}
	}

	svc := &Service{G: git.NewWithRunner(logCacheRunner{gitDir: gitDir}), Store: mem}
	report, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if report.Trunk != "main" {
		t.Fatalf("Trunk = %q, want main", report.Trunk)
	}
	if len(report.Branches) != 2 {
		t.Fatalf("status rows = %+v, want two PR-backed branches", report.Branches)
	}
	if report.Branches[0].Branch != "feat-a" || report.Branches[0].Checks != gh.ChecksPass || report.Branches[0].Mergeable != gh.MergeMergeable {
		t.Fatalf("feat-a row wrong: %+v", report.Branches[0])
	}
	if report.Branches[1].Branch != "feat-b" || !report.Branches[1].Draft || report.Branches[1].Mergeable != gh.MergeConflicting {
		t.Fatalf("feat-b row wrong: %+v", report.Branches[1])
	}

	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer db.Close()
	snap, ok, err := db.PRStatusSnapshot(ctx, 11)
	if err != nil {
		t.Fatalf("PRStatusSnapshot: %v", err)
	}
	if !ok {
		t.Fatalf("expected status snapshot for PR 11")
	}
	if snap.Checks != string(gh.ChecksFail) || snap.Mergeable != string(gh.MergeConflicting) || !snap.Draft {
		t.Fatalf("cached status snapshot wrong: %+v", snap)
	}
}

func TestStatusRequiresGhPreflight(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}

	oldPreflight := statusPreflightCheck
	oldClient := newStatusClient
	defer func() {
		statusPreflightCheck = oldPreflight
		newStatusClient = oldClient
	}()
	wantErr := errors.New("no gh")
	statusPreflightCheck = func(context.Context) error { return wantErr }
	newStatusClient = func() prStatusClient {
		t.Fatal("newStatusClient should not be called when preflight fails")
		return nil
	}

	svc := &Service{G: git.NewWithRunner(logCacheRunner{gitDir: t.TempDir()}), Store: mem}
	if _, err := svc.Status(ctx); !errors.Is(err, wantErr) {
		t.Fatalf("Status error = %v, want %v", err, wantErr)
	}
}

func TestStatusKeepsPartialRowsOnFetchError(t *testing.T) {
	ctx := context.Background()
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", PR: 10}); err != nil {
		t.Fatalf("SetBranch feat-a: %v", err)
	}

	oldPreflight := statusPreflightCheck
	oldClient := newStatusClient
	defer func() {
		statusPreflightCheck = oldPreflight
		newStatusClient = oldClient
	}()
	statusPreflightCheck = func(context.Context) error { return nil }
	newStatusClient = func() prStatusClient {
		return fakeStatusClient{errs: map[int]error{10: errors.New("rate limited")}}
	}

	svc := &Service{G: git.NewWithRunner(logCacheRunner{gitDir: t.TempDir()}), Store: mem}
	report, err := svc.Status(ctx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(report.Branches) != 1 || report.Branches[0].Err == "" {
		t.Fatalf("expected partial error row, got %+v", report.Branches)
	}
}

func TestRefreshCachedPRStatusesBestEffort(t *testing.T) {
	ctx := context.Background()
	gitDir := filepath.Join(t.TempDir(), ".git")
	mem := memory.New()
	if err := mem.SetRepo(ctx, store.RepoMeta{Trunk: "main", Version: 1}); err != nil {
		t.Fatalf("SetRepo: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-a", store.BranchMeta{Parent: "main", PR: 10}); err != nil {
		t.Fatalf("SetBranch feat-a: %v", err)
	}
	if err := mem.SetBranch(ctx, "feat-b", store.BranchMeta{Parent: "main", PR: 11}); err != nil {
		t.Fatalf("SetBranch feat-b: %v", err)
	}
	g, err := stack.Load(ctx, mem)
	if err != nil {
		t.Fatalf("stack.Load: %v", err)
	}
	svc := &Service{G: git.NewWithRunner(logCacheRunner{gitDir: gitDir}), Store: mem}
	svc.refreshCachedPRStatuses(ctx, g, fakeStatusClient{
		byPR: map[int]gh.PRStatus{
			10: {Number: 10, Checks: gh.ChecksPass, Mergeable: gh.MergeMergeable, State: gh.PRStateOpen},
		},
		errs: map[int]error{11: errors.New("rate limited")},
	})

	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	defer db.Close()
	if snap, ok, err := db.PRStatusSnapshot(ctx, 10); err != nil || !ok || snap.Checks != string(gh.ChecksPass) {
		t.Fatalf("PR 10 snapshot = %+v ok=%v err=%v, want cached pass", snap, ok, err)
	}
	if _, ok, err := db.PRStatusSnapshot(ctx, 11); err != nil || ok {
		t.Fatalf("PR 11 snapshot ok=%v err=%v, want missing after fetch error", ok, err)
	}
}

func TestCachedPRStatusBranchesReturnsCachedRowsOnly(t *testing.T) {
	ctx := context.Background()
	gitDir := filepath.Join(t.TempDir(), ".git")
	svc := &Service{G: git.NewWithRunner(logCacheRunner{gitDir: gitDir}), Store: memory.New()}

	db, err := cache.Open(ctx, gitDir)
	if err != nil {
		t.Fatalf("cache.Open: %v", err)
	}
	if err := db.PutPRStatusSnapshot(ctx, cache.PRStatusSnapshot{
		Number:    10,
		Checks:    string(gh.ChecksFail),
		Mergeable: string(gh.MergeConflicting),
		State:     string(gh.PRStateOpen),
		Draft:     true,
	}); err != nil {
		t.Fatalf("PutPRStatusSnapshot: %v", err)
	}
	db.Close()

	got := svc.cachedPRStatusBranches(ctx, []CheckoutItem{
		{Branch: "main", IsTrunk: true},
		{Branch: "feat-a", PR: 10},
		{Branch: "feat-b", PR: 11},
	})
	if len(got) != 1 {
		t.Fatalf("cached status rows = %+v, want one row", got)
	}
	row := got["feat-a"]
	if row.PR != 10 || row.Checks != gh.ChecksFail || row.Mergeable != gh.MergeConflicting || !row.Draft {
		t.Fatalf("feat-a cached status = %+v, want cached PR 10 status", row)
	}
	if _, ok := got["feat-b"]; ok {
		t.Fatalf("feat-b should be absent because PR 11 was not cached")
	}
}
