package cockpit

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/bluegardenproject/stac-man/internal/service"
)

// actionResult is the message every cockpit-dispatched local action
// resolves to. The dashboard reads it to update the status line and
// to decide whether to chain a snapshot reload (success) or hand off
// to the conflict resolver (paused).
//
// Holding all three of Verb, Branch, and the original Err on the
// same struct keeps the post-action plumbing branchless: one switch
// in Update covers success, plain failure, and paused. The
// alternative — separate message types per outcome — duplicated the
// dispatch logic at every action site without buying extra type
// safety because the verbs are still strings.
type actionResult struct {
	// Verb is a short imperative label ("checkout", "restack", …)
	// used by the status line. Pulled from the binding's Help so
	// the help overlay and the status line stay in sync.
	Verb string
	// Branch is the operand for cursor-targeted actions; empty for
	// HEAD-targeted ones (modify/fold/absorb/undo) where the user's
	// current git branch is the implicit subject.
	Branch string
	// Err is whatever the underlying *service.Service call
	// returned. Nil means success.
	Err error
	// Paused, when non-nil, is Err type-asserted to a paused-rebase
	// error. Held alongside Err (rather than replacing it) so the
	// status line can still render the textual description.
	Paused *service.PausedError
}

// runActionCmd wraps a service-call closure into a tea.Cmd that
// resolves to an actionResult. Centralising this stops every action
// site from re-implementing the errors.As + actionResult plumbing.
//
// fn is intentionally argument-less: callers close over the model's
// ctx and svc so we don't re-thread them through every helper. The
// closure is invoked on bubbletea's command goroutine, so blocking
// service calls don't stall the render loop.
func runActionCmd(verb, branch string, fn func() error) tea.Cmd {
	return func() tea.Msg {
		err := fn()
		res := actionResult{Verb: verb, Branch: branch, Err: err}
		if err != nil {
			var p *service.PausedError
			if errors.As(err, &p) {
				res.Paused = p
			}
		}
		return res
	}
}

// Action helpers per binding. Each one builds the runActionCmd with
// the right verb label + service call. They sit on Model so they can
// read m.ctx and m.svc directly, mirroring how loadDetailCmd /
// loadSnapshotCmd are constructed.
//
// Some actions are cursor-targeted (require a non-empty branch),
// others operate on the current git HEAD. Both cases route through
// the same helper so the status-line plumbing is uniform.

func (m Model) checkoutCmd(branch string) tea.Cmd {
	return runActionCmd(m.keys.Checkout.Help, branch, func() error {
		return m.svc.Checkout(m.ctx, branch)
	})
}

func (m Model) restackCmd(branch string) tea.Cmd {
	return runActionCmd(m.keys.Restack.Help, branch, func() error {
		return m.svc.Restack(m.ctx, branch)
	})
}

func (m Model) trackCmd(branch string) tea.Cmd {
	return runActionCmd(m.keys.Track.Help, branch, func() error {
		_, err := m.svc.Track(m.ctx, service.TrackOptions{Branch: branch})
		return err
	})
}

// untrackCmd never reparents children of the untracked branch — the
// CLI's `sm untrack <branch>` defaults to false too, and surprising
// the user with implicit reparenting from a one-key TUI press would
// be a recipe for "where did my children go".
func (m Model) untrackCmd(branch string) tea.Cmd {
	return runActionCmd(m.keys.Untrack.Help, branch, func() error {
		return m.svc.Untrack(m.ctx, branch, false)
	})
}

// modifyCmd amends the current branch's HEAD commit without
// changing the message (Modify uses --no-edit when Message is
// empty). Stage-all is intentionally off so the TUI never silently
// stages files the user hadn't queued; this matches the CLI default.
func (m Model) modifyCmd() tea.Cmd {
	return runActionCmd(m.keys.Modify.Help, "", func() error {
		return m.svc.Modify(m.ctx, service.ModifyOptions{Amend: true})
	})
}

// foldCmd squashes the current branch into its parent. Fold takes
// an optional message; passing "" lets git's default merge-commit
// message stand, which is appropriate for the one-key path.
func (m Model) foldCmd() tea.Cmd {
	return runActionCmd(m.keys.Fold.Help, "", func() error {
		return m.svc.Fold(m.ctx, "")
	})
}

// absorbCmd routes uncommitted hunks back into ancestor commits.
// No base override — Service.Absorb auto-resolves the lowest
// tracked ancestor, which is what `sm absorb` with no flags does.
func (m Model) absorbCmd() tea.Cmd {
	return runActionCmd(m.keys.Absorb.Help, "", func() error {
		_, err := m.svc.Absorb(m.ctx, service.AbsorbOptions{})
		return err
	})
}

// undoCmd reverts the most recent stac-man op. dryRun=false because
// the cockpit user pressed `u` to actually undo, not preview.
func (m Model) undoCmd() tea.Cmd {
	return runActionCmd(m.keys.Undo.Help, "", func() error {
		_, err := m.svc.Undo(m.ctx, false)
		return err
	})
}

// statusLine renders a one-line summary of the most recent action
// outcome, or "" when no action has run yet. Three colour bands
// distinguish the cases at a glance:
//
//	OK   — success
//	Warn — paused (will route to the conflict resolver in todo 7)
//	Fail — plain failure
//
// Verb / Branch come straight from the actionResult so this stays
// the only place that formats the prose; tests assert against the
// substrings produced here.
func statusLine(res *actionResult) string {
	if res == nil {
		return ""
	}
	target := res.Branch
	switch {
	case res.Paused != nil:
		// Use the paused error's branch when the action was
		// HEAD-targeted (Branch is "") so the status line still
		// names the conflicting branch.
		if target == "" {
			target = res.Paused.Branch
		}
		return fmt.Sprintf("paused: %s on %s — resolve, then continue", res.Verb, target)
	case res.Err != nil:
		if target != "" {
			return fmt.Sprintf("%s %s failed: %v", res.Verb, target, res.Err)
		}
		return fmt.Sprintf("%s failed: %v", res.Verb, res.Err)
	default:
		if target != "" {
			return fmt.Sprintf("%s %s ✓", res.Verb, target)
		}
		return fmt.Sprintf("%s ✓", res.Verb)
	}
}
