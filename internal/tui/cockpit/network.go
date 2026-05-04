package cockpit

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

// networkFinishedMsg is delivered after a tea.ExecProcess wrapping
// a sm-subcommand subprocess returns. It carries the verb (so the
// status line can name the action) and any exit error from the
// subprocess.
//
// Held separately from actionResult because the network-action
// path doesn't have a *PausedError to surface — paused state from a
// shell-out lands on disk under .git/restack.json and is picked up
// by the next snapshot reload, which then routes the user into the
// conflict resolver via routeAfterPostNetwork.
type networkFinishedMsg struct {
	// Verb is the binding's help label (submit / sync / land) so
	// status-line text stays aligned with the keymap.
	Verb string
	// Err is the subprocess exit error, nil on a clean exit.
	Err error
}

// resolveSelfBinary returns the path to the currently-running sm
// binary so we can re-invoke it for shell-out actions. Pulled out
// as a package var so tests can override the resolver without
// shelling out to a real os.Executable() call.
//
// We prefer os.Executable() over os.Args[0] because the latter can
// be a relative path or even a symlink target, and the sm binary
// must work from anywhere in the user's filesystem.
var resolveSelfBinary = func() (string, error) {
	return os.Executable()
}

// networkExecCmd builds an exec.Cmd that re-invokes the running sm
// binary with the given subcommand args, wired to the user's real
// stdio. tea.ExecProcess takes the cmd, releases the alt-screen,
// runs it, then reattaches — which is exactly what we need for
// gh's auth prompts and the polished CLI output renderers.
//
// Returns an error if the self-binary can't be resolved; the caller
// surfaces this as a networkFinishedMsg with Err set so the user
// sees something rather than a silent no-op.
func networkExecCmd(verb string, args ...string) tea.Cmd {
	bin, err := resolveSelfBinary()
	if err != nil {
		return func() tea.Msg {
			return networkFinishedMsg{Verb: verb, Err: fmt.Errorf("locating sm binary: %w", err)}
		}
	}
	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return networkFinishedMsg{Verb: verb, Err: err}
	})
}

// Per-binding helpers. Each translates a cockpit keypress into the
// matching `sm <subcommand>` invocation. Verb labels come from the
// keymap so renaming a binding propagates here automatically.
//
// Defaults mirror the CLI defaults rather than the most-aggressive
// flag set: a one-key press shouldn't, for instance, push every
// branch in the stack when the user only wanted the current one.
// Cockpit users who want `--stack` can fall back to the palette
// (todo `palette`) once it lands.

func (m Model) submitCmd() tea.Cmd {
	return networkExecCmd(m.keys.Submit.Help, "submit")
}

func (m Model) syncCmd() tea.Cmd {
	return networkExecCmd(m.keys.Sync.Help, "sync")
}

func (m Model) landCmd() tea.Cmd {
	return networkExecCmd(m.keys.Land.Help, "land")
}
