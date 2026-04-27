package restack

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// State captures everything we need to resume an interrupted restack.
// It's persisted as JSON to .git/stac-man/restack.json so a single
// process restart, conflict pause, or `sm abort` can pick up where
// the previous invocation left off.
type State struct {
	// Origin describes which command created this state. Used purely
	// for human-readable error messages.
	Origin string `json:"origin"`
	// Pending is the queue of branches still to restack, in topo order.
	// The branch currently being rebased (the one that conflicted) is
	// at index 0.
	Pending []PendingBranch `json:"pending"`
	// SavedTip is the SHA of the branch we started from before the
	// restack began, so `sm abort` can hard-reset back to it.
	SavedTip string `json:"saved_tip,omitempty"`
	// SavedBranch is the branch HEAD was on when restack started, used
	// to restore checkout on abort.
	SavedBranch string `json:"saved_branch,omitempty"`
}

// PendingBranch is one entry in the resume queue.
type PendingBranch struct {
	Name      string `json:"name"`
	OldParent string `json:"old_parent"`
	NewParent string `json:"new_parent"`
	OldSHA    string `json:"old_sha"`
}

// stateFilename is the on-disk filename relative to the git dir.
const stateFilename = "stac-man/restack.json"

// Save writes State atomically into <gitDir>/stac-man/restack.json.
func Save(gitDir string, st *State) error {
	dir := filepath.Join(gitDir, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	full := filepath.Join(gitDir, stateFilename)
	tmp, err := os.CreateTemp(dir, ".restack-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(st); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), full)
}

// Load reads the persisted resume state, returning ok=false (with no
// error) when the file is absent.
func Load(gitDir string) (*State, bool, error) {
	full := filepath.Join(gitDir, stateFilename)
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, false, fmt.Errorf("parsing restack state: %w", err)
	}
	return &st, true, nil
}

// Clear removes the persisted state. Idempotent.
func Clear(gitDir string) error {
	full := filepath.Join(gitDir, stateFilename)
	err := os.Remove(full)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
