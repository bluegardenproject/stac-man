// Package history persists a bounded log of stac-man's mutating
// operations so `sm undo` can roll the most recent one back. Entries
// live under .git/stac-man/history.json — one file per repo, capped
// at MaxEntries.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MaxEntries is the upper bound on history depth. Older entries are
// dropped on Append.
const MaxEntries = 50

// Snapshot captures everything `sm undo` needs to restore a single
// branch to its pre-op state.
type Snapshot struct {
	Branch     string `json:"branch"`
	ParentName string `json:"parentName"`
	ParentSHA  string `json:"parentSHA"`
	Tip        string `json:"tip"`
	PR         int    `json:"pr,omitempty"`
	// Tracked is whether the branch existed in the stack graph at
	// snapshot time. False means undo should untrack the branch
	// (rather than re-track it with this snapshot's metadata).
	Tracked bool `json:"tracked"`
	// Existed is whether the local ref existed at all. False means the
	// branch was created by the op and undo should delete it.
	Existed bool `json:"existed"`
}

// Entry is one undo-able step.
type Entry struct {
	ID    string              `json:"id"`
	Time  time.Time           `json:"time"`
	Op    string              `json:"op"`
	Notes string              `json:"notes,omitempty"`
	// Before captures the pre-op state of every branch the op touched.
	Before map[string]Snapshot `json:"before"`
	// HEAD records the branch that was checked out before the op so
	// undo can hop back.
	HEAD string `json:"head,omitempty"`
}

// Path returns the on-disk location for the history file given the
// repo's .git directory.
func Path(gitDir string) string {
	return filepath.Join(gitDir, "stac-man", "history.json")
}

// Load reads the history file. Returns an empty slice + nil error if
// the file doesn't exist; treat the error path as "history is broken".
func Load(gitDir string) ([]Entry, error) {
	p := Path(gitDir)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", p, err)
	}
	if len(data) == 0 {
		return nil, nil
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", p, err)
	}
	return entries, nil
}

// Save writes entries to disk, creating the parent directory as
// needed. Atomic via temp-file + rename.
func Save(gitDir string, entries []Entry) error {
	p := Path(gitDir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Append adds an entry to the front of history (newest-first), capping
// at MaxEntries.
func Append(gitDir string, entry Entry) error {
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("h-%d", time.Now().UnixNano())
	}
	if entry.Time.IsZero() {
		entry.Time = time.Now().UTC()
	}
	entries, err := Load(gitDir)
	if err != nil {
		return err
	}
	entries = append([]Entry{entry}, entries...)
	if len(entries) > MaxEntries {
		entries = entries[:MaxEntries]
	}
	return Save(gitDir, entries)
}

// Pop removes and returns the most recent entry. ok=false when the
// history is empty.
func Pop(gitDir string) (Entry, bool, error) {
	entries, err := Load(gitDir)
	if err != nil {
		return Entry{}, false, err
	}
	if len(entries) == 0 {
		return Entry{}, false, nil
	}
	head := entries[0]
	rest := entries[1:]
	if err := Save(gitDir, rest); err != nil {
		return Entry{}, false, err
	}
	return head, true, nil
}

// Peek returns the most recent entry without removing it. Used by
// `sm undo --dry-run`.
func Peek(gitDir string) (Entry, bool, error) {
	entries, err := Load(gitDir)
	if err != nil {
		return Entry{}, false, err
	}
	if len(entries) == 0 {
		return Entry{}, false, nil
	}
	return entries[0], true, nil
}

// CtxAppend is a convenience wrapper for use inside service methods.
// It accepts a context purely for shape symmetry — the underlying
// file I/O is synchronous.
func CtxAppend(_ context.Context, gitDir string, entry Entry) error {
	return Append(gitDir, entry)
}
