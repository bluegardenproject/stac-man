// Package cache owns stac-man's repo-local SQLite cache.
//
// The cache lives under the real git directory returned by
// `git rev-parse --git-dir`, so normal clones, worktrees, and
// submodules all keep their own local-only derived state.
package cache

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	filename             = "cache.db"
	currentSchemaVersion = 1
)

// ErrMissingGitDir is returned when callers try to open the cache
// without first resolving the repo's git dir.
var ErrMissingGitDir = errors.New("git dir required")

// DB wraps the SQLite handle so callers do not depend on database/sql
// details or the selected driver.
type DB struct {
	path string
	sql  *sql.DB
}

// PRSnapshot is the cached, repo-local summary of a GitHub pull
// request. It deliberately excludes volatile checks and mergeability,
// which live in PRStatusSnapshot.
type PRSnapshot struct {
	Branch    string
	Number    int
	State     string
	Draft     bool
	Title     string
	URL       string
	Base      string
	Head      string
	FetchedAt time.Time
}

// PRStatusSnapshot is the cached result of an explicit live GitHub
// status refresh. It is keyed by PR number because checks and
// mergeability belong to the PR, not the local branch name.
type PRStatusSnapshot struct {
	Number    int
	Checks    string
	Mergeable string
	State     string
	Draft     bool
	FetchedAt time.Time
}

// Path returns the repo-local cache path for a git dir. Callers should
// pass the value returned by git rev-parse --git-dir, not assume ".git".
func Path(gitDir string) string {
	return filepath.Join(gitDir, "stac-man", filename)
}

// Open opens the repo-local cache, creating parent directories and
// applying schema migrations. If the existing database is corrupt, it
// is moved aside and rebuilt because this file is only a cache.
func Open(ctx context.Context, gitDir string) (*DB, error) {
	if gitDir == "" {
		return nil, ErrMissingGitDir
	}
	p := Path(gitDir)
	db, err := open(ctx, p)
	if err == nil {
		return db, nil
	}
	if !isCorrupt(err) {
		return nil, err
	}
	if moveErr := moveCorrupt(p); moveErr != nil {
		return nil, fmt.Errorf("moving corrupt cache: %w", moveErr)
	}
	return open(ctx, p)
}

func open(ctx context.Context, path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	handle, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db := &DB{path: path, sql: handle}
	if err := db.configure(ctx); err != nil {
		_ = handle.Close()
		return nil, err
	}
	if err := db.migrate(ctx); err != nil {
		_ = handle.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) configure(ctx context.Context) error {
	if _, err := db.sql.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}
	if _, err := db.sql.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	return nil
}

func (db *DB) migrate(ctx context.Context) error {
	tx, err := db.sql.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS pr_snapshot (
			branch TEXT PRIMARY KEY,
			pr_number INTEGER NOT NULL,
			state TEXT NOT NULL DEFAULT '',
			draft INTEGER NOT NULL DEFAULT 0,
			title TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			base TEXT NOT NULL DEFAULT '',
			head TEXT NOT NULL DEFAULT '',
			fetched_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS pr_snapshot_number_idx
			ON pr_snapshot(pr_number)`,
		`CREATE TABLE IF NOT EXISTS pr_status_snapshot (
			pr_number INTEGER PRIMARY KEY,
			checks TEXT NOT NULL DEFAULT '',
			mergeable TEXT NOT NULL DEFAULT '',
			state TEXT NOT NULL DEFAULT '',
			draft INTEGER NOT NULL DEFAULT 0,
			fetched_at TEXT NOT NULL
		)`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	var version int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return err
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("cache schema version %d is newer than supported version %d", version, currentSchemaVersion)
	}
	if version < currentSchemaVersion {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`,
			currentSchemaVersion,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Close closes the underlying database handle.
func (db *DB) Close() error {
	if db == nil || db.sql == nil {
		return nil
	}
	return db.sql.Close()
}

// Path returns the full path to this opened cache database.
func (db *DB) Path() string {
	if db == nil {
		return ""
	}
	return db.path
}

// SchemaVersion returns the highest applied schema version.
func (db *DB) SchemaVersion(ctx context.Context) (int, error) {
	if db == nil || db.sql == nil {
		return 0, sql.ErrConnDone
	}
	var version int
	err := db.sql.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version)
	return version, err
}

// PutPRSnapshot stores the latest PR summary for a branch. Callers pass
// a snapshot only after they have already fetched or created the PR via
// GitHub; read-only commands should not use this to refresh data.
func (db *DB) PutPRSnapshot(ctx context.Context, snap PRSnapshot) error {
	if db == nil || db.sql == nil {
		return sql.ErrConnDone
	}
	if snap.Branch == "" || snap.Number == 0 {
		return nil
	}
	if snap.FetchedAt.IsZero() {
		snap.FetchedAt = time.Now().UTC()
	}
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO pr_snapshot (
			branch, pr_number, state, draft, title, url, base, head, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(branch) DO UPDATE SET
			pr_number = excluded.pr_number,
			state = excluded.state,
			draft = excluded.draft,
			title = excluded.title,
			url = excluded.url,
			base = excluded.base,
			head = excluded.head,
			fetched_at = excluded.fetched_at
	`, snap.Branch, snap.Number, snap.State, boolInt(snap.Draft), snap.Title, snap.URL, snap.Base, snap.Head, formatTime(snap.FetchedAt))
	return err
}

// PRSnapshots returns cached PR summaries keyed by branch. Missing
// branches are absent from the map.
func (db *DB) PRSnapshots(ctx context.Context, branches []string) (map[string]PRSnapshot, error) {
	out := map[string]PRSnapshot{}
	if db == nil || db.sql == nil {
		return out, sql.ErrConnDone
	}
	if len(branches) == 0 {
		return out, nil
	}
	for _, branch := range branches {
		if branch == "" {
			continue
		}
		snap, ok, err := db.PRSnapshot(ctx, branch)
		if err != nil {
			return nil, err
		}
		if ok {
			out[branch] = snap
		}
	}
	return out, nil
}

// PRSnapshot returns the cached PR summary for one branch.
func (db *DB) PRSnapshot(ctx context.Context, branch string) (PRSnapshot, bool, error) {
	if db == nil || db.sql == nil {
		return PRSnapshot{}, false, sql.ErrConnDone
	}
	var snap PRSnapshot
	var draft int
	var fetchedAt string
	err := db.sql.QueryRowContext(ctx, `
		SELECT branch, pr_number, state, draft, title, url, base, head, fetched_at
		FROM pr_snapshot
		WHERE branch = ?
	`, branch).Scan(
		&snap.Branch,
		&snap.Number,
		&snap.State,
		&draft,
		&snap.Title,
		&snap.URL,
		&snap.Base,
		&snap.Head,
		&fetchedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PRSnapshot{}, false, nil
	}
	if err != nil {
		return PRSnapshot{}, false, err
	}
	parsed, err := parseTime(fetchedAt)
	if err != nil {
		return PRSnapshot{}, false, err
	}
	snap.Draft = draft != 0
	snap.FetchedAt = parsed
	return snap, true, nil
}

// PutPRStatusSnapshot stores the latest live GitHub status for a PR.
func (db *DB) PutPRStatusSnapshot(ctx context.Context, snap PRStatusSnapshot) error {
	if db == nil || db.sql == nil {
		return sql.ErrConnDone
	}
	if snap.Number == 0 {
		return nil
	}
	if snap.FetchedAt.IsZero() {
		snap.FetchedAt = time.Now().UTC()
	}
	_, err := db.sql.ExecContext(ctx, `
		INSERT INTO pr_status_snapshot (
			pr_number, checks, mergeable, state, draft, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(pr_number) DO UPDATE SET
			checks = excluded.checks,
			mergeable = excluded.mergeable,
			state = excluded.state,
			draft = excluded.draft,
			fetched_at = excluded.fetched_at
	`, snap.Number, snap.Checks, snap.Mergeable, snap.State, boolInt(snap.Draft), formatTime(snap.FetchedAt))
	return err
}

// PRStatusSnapshot returns the cached live GitHub status for one PR.
func (db *DB) PRStatusSnapshot(ctx context.Context, number int) (PRStatusSnapshot, bool, error) {
	if db == nil || db.sql == nil {
		return PRStatusSnapshot{}, false, sql.ErrConnDone
	}
	var snap PRStatusSnapshot
	var draft int
	var fetchedAt string
	err := db.sql.QueryRowContext(ctx, `
		SELECT pr_number, checks, mergeable, state, draft, fetched_at
		FROM pr_status_snapshot
		WHERE pr_number = ?
	`, number).Scan(
		&snap.Number,
		&snap.Checks,
		&snap.Mergeable,
		&snap.State,
		&draft,
		&fetchedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return PRStatusSnapshot{}, false, nil
	}
	if err != nil {
		return PRStatusSnapshot{}, false, err
	}
	parsed, err := parseTime(fetchedAt)
	if err != nil {
		return PRStatusSnapshot{}, false, err
	}
	snap.Draft = draft != 0
	snap.FetchedAt = parsed
	return snap, true, nil
}

// DeletePRStatusSnapshots removes volatile status rows for PRs whose
// GitHub state is known to have gone stale.
func (db *DB) DeletePRStatusSnapshots(ctx context.Context, numbers []int) error {
	if db == nil || db.sql == nil {
		return sql.ErrConnDone
	}
	for _, number := range numbers {
		if number == 0 {
			continue
		}
		if _, err := db.sql.ExecContext(ctx, `DELETE FROM pr_status_snapshot WHERE pr_number = ?`, number); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}

func isCorrupt(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "file is not a database") ||
		strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "database is malformed")
}

func moveCorrupt(path string) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	corruptPath := fmt.Sprintf("%s.corrupt.%d", path, time.Now().UnixNano())
	if err := os.Rename(path, corruptPath); err == nil {
		return nil
	}
	return os.Remove(path)
}
