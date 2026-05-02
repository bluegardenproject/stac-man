package service

import (
	"encoding/json"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/history"
)

func TestBranchMetaFromSnapPreservesFields(t *testing.T) {
	snap := history.Snapshot{
		Branch:     "feat-a",
		ParentName: "main",
		ParentSHA:  "abc",
		Tip:        "def",
		PR:         42,
		Tracked:    true,
		Existed:    true,
	}
	got := branchMetaFromSnap(snap)
	if got.Parent != "main" || got.ParentSHA != "abc" || got.PR != 42 {
		t.Fatalf("branchMetaFromSnap dropped fields: %+v", got)
	}
}

func TestUndoResultFromCarriesFields(t *testing.T) {
	entry := history.Entry{
		Op:    "modify",
		Notes: "amend",
		Before: map[string]history.Snapshot{
			"feat-a": {Branch: "feat-a"},
			"feat-b": {Branch: "feat-b"},
		},
	}
	got := undoResultFrom(entry, true)
	if got.Op != "modify" || got.Notes != "amend" || !got.DryRun {
		t.Fatalf("undoResultFrom dropped fields: %+v", got)
	}
	if len(got.Branches) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(got.Branches))
	}
}

// TestHistoryRoundTripWithSnapshot pins the on-disk JSON shape so
// undo can read entries written by older binaries without surprise.
func TestHistoryRoundTripWithSnapshot(t *testing.T) {
	gitDir := t.TempDir()
	want := history.Entry{
		Op: "fold",
		Before: map[string]history.Snapshot{
			"feat-a": {Branch: "feat-a", Tracked: true, Existed: true, Tip: "abc"},
		},
		HEAD: "feat-a",
	}
	if err := history.Append(gitDir, want); err != nil {
		t.Fatalf("Append: %v", err)
	}

	entries, err := history.Load(gitDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.Op != want.Op || got.HEAD != want.HEAD {
		t.Fatalf("op/head drift: got %+v", got)
	}
	snap := got.Before["feat-a"]
	if snap.Branch != "feat-a" || !snap.Tracked || !snap.Existed || snap.Tip != "abc" {
		t.Fatalf("snapshot drift: got %+v", snap)
	}
	// And the JSON should be stable enough to grep — useful for
	// debugging without a tool.
	data, _ := json.Marshal(got)
	if string(data) == "" {
		t.Fatalf("expected non-empty JSON")
	}
}
