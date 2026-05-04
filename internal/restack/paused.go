package restack

import "context"

// PausedInfo is the engine-level view of a paused stac-man operation.
// It bundles the on-disk State (origin, pending queue, the branch
// HEAD was on when the op started) with the live conflict paths
// from the git index so callers — chiefly the interactive cockpit's
// conflict resolver — can render a complete picture from a single
// call instead of stitching together a file read and a `git diff`.
//
// The shape is the contract `service.PausedSnapshot` aliases, so
// adding a field here propagates to every consumer; aim to keep it
// minimal and tightly scoped to what a resolver UI needs.
type PausedInfo struct {
	// Origin is the command that created the paused state ("restack",
	// "sync", "modify", ...). Mirrors restack.State.Origin.
	Origin string
	// Branch is the branch the rebase paused on (the head of the
	// pending queue). Empty when the on-disk queue is empty —
	// shouldn't happen in practice but is handled defensively so a
	// corrupt-but-present state file doesn't panic the resolver.
	Branch string
	// Pending lists every branch still to process in topo order,
	// including Branch as the first entry.
	Pending []string
	// SavedBranch is the branch HEAD was on when the paused op
	// started. The resolver shows it in the "abort returns you to
	// <branch>" hint.
	SavedBranch string
	// ConflictPaths lists the unmerged files in the index right now.
	// Empty when the user has staged every resolution but has not
	// yet run `sm continue`. Always populated from a live
	// `git diff --name-only --diff-filter=U` so the cockpit doesn't
	// show a stale snapshot.
	ConflictPaths []string
}

// Paused returns the current paused state, or (nil, false, nil) when
// no paused operation exists. Splitting the boolean out of the
// pointer lets callers distinguish "no paused op" (cleanly absent)
// from "paused op present but the queue is empty" (the latter would
// be a real corruption signal worth flagging in the UI).
//
// A failure to read the git dir or list conflict paths is returned
// as a real error: silently swallowing either would mask a stuck
// restack from the resolver, which is precisely the failure mode
// this method exists to surface.
func (e *Engine) Paused(ctx context.Context) (*PausedInfo, bool, error) {
	gitDir, err := e.G.GitDir(ctx)
	if err != nil {
		return nil, false, err
	}
	st, ok, err := Load(gitDir)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}

	info := &PausedInfo{
		Origin:      st.Origin,
		SavedBranch: st.SavedBranch,
	}
	if len(st.Pending) > 0 {
		info.Branch = st.Pending[0].Name
		info.Pending = make([]string, 0, len(st.Pending))
		for _, p := range st.Pending {
			info.Pending = append(info.Pending, p.Name)
		}
	}

	paths, err := e.G.ConflictPaths(ctx)
	if err != nil {
		return nil, false, err
	}
	info.ConflictPaths = paths

	return info, true, nil
}
