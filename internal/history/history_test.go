package history

import (
	"path/filepath"
	"testing"
)

func TestAppendAndPopRoundTrip(t *testing.T) {
	gitDir := t.TempDir()
	if err := Append(gitDir, Entry{Op: "first"}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := Append(gitDir, Entry{Op: "second"}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, ok, err := Pop(gitDir)
	if err != nil || !ok {
		t.Fatalf("Pop: ok=%v err=%v", ok, err)
	}
	if got.Op != "second" {
		t.Fatalf("expected newest entry first, got %q", got.Op)
	}

	got, ok, err = Pop(gitDir)
	if err != nil || !ok {
		t.Fatalf("Pop: ok=%v err=%v", ok, err)
	}
	if got.Op != "first" {
		t.Fatalf("expected first, got %q", got.Op)
	}

	if _, ok, _ := Pop(gitDir); ok {
		t.Fatalf("expected empty history after both pops")
	}
}

func TestPeekDoesNotConsume(t *testing.T) {
	gitDir := t.TempDir()
	_ = Append(gitDir, Entry{Op: "x"})
	if _, ok, _ := Peek(gitDir); !ok {
		t.Fatalf("Peek: expected entry")
	}
	if _, ok, _ := Peek(gitDir); !ok {
		t.Fatalf("Peek should be idempotent")
	}
	if _, ok, _ := Pop(gitDir); !ok {
		t.Fatalf("Pop after Peek should still find the entry")
	}
}

func TestAppendCapsAtMaxEntries(t *testing.T) {
	gitDir := t.TempDir()
	for i := 0; i < MaxEntries+10; i++ {
		if err := Append(gitDir, Entry{Op: "op"}); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	entries, err := Load(gitDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != MaxEntries {
		t.Fatalf("expected %d entries after cap, got %d", MaxEntries, len(entries))
	}
}

func TestLoadEmptyReturnsNil(t *testing.T) {
	entries, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load on empty dir: %v", err)
	}
	if entries != nil {
		t.Fatalf("expected nil entries, got %v", entries)
	}
}

func TestPathPlacement(t *testing.T) {
	got := Path("/tmp/repo/.git")
	want := filepath.Join("/tmp/repo/.git", "stac-man", "history.json")
	if got != want {
		t.Fatalf("Path: got %q want %q", got, want)
	}
}

func TestSnapshotJSONOmitsEmpty(t *testing.T) {
	gitDir := t.TempDir()
	entry := Entry{
		Op: "create",
		Before: map[string]Snapshot{
			"feat-x": {Branch: "feat-x", Existed: false},
		},
	}
	if err := Append(gitDir, entry); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, ok, err := Pop(gitDir)
	if err != nil || !ok {
		t.Fatalf("Pop: ok=%v err=%v", ok, err)
	}
	snap, ok := got.Before["feat-x"]
	if !ok {
		t.Fatalf("missing branch in restored entry")
	}
	if snap.Existed || snap.Tracked {
		t.Fatalf("expected zero flags for never-existed branch, got %+v", snap)
	}
}
