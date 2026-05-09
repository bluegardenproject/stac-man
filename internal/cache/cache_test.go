package cache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPathIsUnderGitDir(t *testing.T) {
	gitDir := filepath.Join("tmp", "repo", ".git")
	got := Path(gitDir)
	want := filepath.Join("tmp", "repo", ".git", "stac-man", "cache.db")
	if got != want {
		t.Fatalf("Path = %q, want %q", got, want)
	}
}

func TestOpenCreatesSchema(t *testing.T) {
	ctx := context.Background()
	gitDir := filepath.Join(t.TempDir(), ".git")
	db, err := Open(ctx, gitDir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if db.Path() != Path(gitDir) {
		t.Fatalf("db.Path = %q, want %q", db.Path(), Path(gitDir))
	}
	if _, err := os.Stat(Path(gitDir)); err != nil {
		t.Fatalf("cache file was not created: %v", err)
	}
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}

	for _, table := range []string{"schema_migrations", "pr_snapshot", "pr_status_snapshot"} {
		var name string
		err := db.sql.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
			table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestOpenMovesCorruptCacheAside(t *testing.T) {
	ctx := context.Background()
	gitDir := filepath.Join(t.TempDir(), ".git")
	path := Path(gitDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	db, err := Open(ctx, gitDir)
	if err != nil {
		t.Fatalf("Open with corrupt cache: %v", err)
	}
	defer db.Close()
	version, err := db.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version = %d, want %d", version, currentSchemaVersion)
	}

	matches, err := filepath.Glob(path + ".corrupt.*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("corrupt cache backups = %v, want exactly one", matches)
	}
}

func TestOpenRequiresGitDir(t *testing.T) {
	_, err := Open(context.Background(), "")
	if !errors.Is(err, ErrMissingGitDir) {
		t.Fatalf("Open with empty git dir error = %v, want ErrMissingGitDir", err)
	}
}

func TestPRSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	fetchedAt := time.Date(2026, 5, 9, 17, 0, 0, 123, time.UTC)
	in := PRSnapshot{
		Branch:    "feat/cache",
		Number:    42,
		State:     "OPEN",
		Draft:     true,
		Title:     "Cache PR metadata",
		URL:       "https://github.com/acme/widgets/pull/42",
		Base:      "main",
		Head:      "feat/cache",
		FetchedAt: fetchedAt,
	}
	if err := db.PutPRSnapshot(ctx, in); err != nil {
		t.Fatalf("PutPRSnapshot: %v", err)
	}

	got, ok, err := db.PRSnapshot(ctx, "feat/cache")
	if err != nil {
		t.Fatalf("PRSnapshot: %v", err)
	}
	if !ok {
		t.Fatalf("expected cached PR snapshot")
	}
	if got != in {
		t.Fatalf("snapshot round trip:\n got  %+v\n want %+v", got, in)
	}
}

func TestPRSnapshotsReturnsOnlyRequestedCachedBranches(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.PutPRSnapshot(ctx, PRSnapshot{Branch: "feat-a", Number: 1, State: "OPEN"}); err != nil {
		t.Fatalf("PutPRSnapshot feat-a: %v", err)
	}
	if err := db.PutPRSnapshot(ctx, PRSnapshot{Branch: "feat-b", Number: 2, State: "MERGED"}); err != nil {
		t.Fatalf("PutPRSnapshot feat-b: %v", err)
	}

	got, err := db.PRSnapshots(ctx, []string{"feat-a", "feat-c"})
	if err != nil {
		t.Fatalf("PRSnapshots: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("cached snapshots = %+v, want exactly feat-a", got)
	}
	if got["feat-a"].Number != 1 {
		t.Fatalf("feat-a number = %d, want 1", got["feat-a"].Number)
	}
	if _, ok := got["feat-b"]; ok {
		t.Fatalf("feat-b was not requested but returned: %+v", got["feat-b"])
	}
	if _, ok := got["feat-c"]; ok {
		t.Fatalf("feat-c was not cached but returned: %+v", got["feat-c"])
	}
}

func TestPutPRSnapshotIgnoresIncompleteRows(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.PutPRSnapshot(ctx, PRSnapshot{Branch: "", Number: 42}); err != nil {
		t.Fatalf("empty branch should be ignored without error: %v", err)
	}
	if err := db.PutPRSnapshot(ctx, PRSnapshot{Branch: "feat-a", Number: 0}); err != nil {
		t.Fatalf("zero number should be ignored without error: %v", err)
	}
	got, err := db.PRSnapshots(ctx, []string{"feat-a"})
	if err != nil {
		t.Fatalf("PRSnapshots: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("incomplete snapshots should not be persisted: %+v", got)
	}
}

func TestPRStatusSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	fetchedAt := time.Date(2026, 5, 9, 18, 0, 0, 456, time.UTC)
	in := PRStatusSnapshot{
		Number:    42,
		Checks:    "PASS",
		Mergeable: "MERGEABLE",
		State:     "OPEN",
		Draft:     true,
		FetchedAt: fetchedAt,
	}
	if err := db.PutPRStatusSnapshot(ctx, in); err != nil {
		t.Fatalf("PutPRStatusSnapshot: %v", err)
	}

	got, ok, err := db.PRStatusSnapshot(ctx, 42)
	if err != nil {
		t.Fatalf("PRStatusSnapshot: %v", err)
	}
	if !ok {
		t.Fatalf("expected cached PR status snapshot")
	}
	if got != in {
		t.Fatalf("status snapshot round trip:\n got  %+v\n want %+v", got, in)
	}
}

func TestPutPRStatusSnapshotIgnoresZeroNumber(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.PutPRStatusSnapshot(ctx, PRStatusSnapshot{Number: 0, Checks: "FAIL"}); err != nil {
		t.Fatalf("zero number should be ignored without error: %v", err)
	}
	_, ok, err := db.PRStatusSnapshot(ctx, 0)
	if err != nil {
		t.Fatalf("PRStatusSnapshot: %v", err)
	}
	if ok {
		t.Fatalf("zero-number snapshot should not be persisted")
	}
}

func TestDeletePRStatusSnapshots(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), ".git"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, number := range []int{1, 2} {
		if err := db.PutPRStatusSnapshot(ctx, PRStatusSnapshot{Number: number, Checks: "PASS"}); err != nil {
			t.Fatalf("PutPRStatusSnapshot %d: %v", number, err)
		}
	}
	if err := db.DeletePRStatusSnapshots(ctx, []int{0, 1}); err != nil {
		t.Fatalf("DeletePRStatusSnapshots: %v", err)
	}
	if _, ok, err := db.PRStatusSnapshot(ctx, 1); err != nil || ok {
		t.Fatalf("PR 1 snapshot ok=%v err=%v, want deleted without error", ok, err)
	}
	if _, ok, err := db.PRStatusSnapshot(ctx, 2); err != nil || !ok {
		t.Fatalf("PR 2 snapshot ok=%v err=%v, want still cached", ok, err)
	}
}
