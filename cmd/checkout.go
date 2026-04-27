package cmd

import (
	"fmt"
	"os"

	"github.com/philipptpunkt/stac-man/internal/tui"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "checkout [branch]",
		Aliases: []string{"co"},
		Short:   "Switch HEAD to a tracked branch (or trunk)",
		Long: "With an argument, checkout switches HEAD to the named branch. " +
			"Without an argument, on a TTY an interactive picker shows the stack " +
			"tree (arrow keys to move, enter to choose, esc to cancel). When stdin " +
			"or stdout isn't a TTY the same tree is printed for piping.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: branchNameCompletion,
		RunE: func(c *cobra.Command, args []string) error {
			s := newService()
			if len(args) == 1 {
				return s.Checkout(c.Context(), args[0])
			}

			current, items, err := s.CheckoutTree(c.Context())
			if err != nil {
				return err
			}

			// Non-TTY (piped, redirected, or no controlling terminal):
			// print the tree and exit. The picker needs raw stdin and
			// would only emit ANSI noise into a pipe.
			if !stdinIsTTY() || !stdoutIsTTY() {
				return tui.RenderTree(c.OutOrStdout(), "Tracked branches", items)
			}

			res, err := tui.PickBranch("Tracked branches", current, items)
			if err != nil {
				return err
			}
			if res.Cancelled || res.Branch == "" || res.Branch == current {
				return nil
			}
			if err := s.Checkout(c.Context(), res.Branch); err != nil {
				return err
			}
			fmt.Printf("%s now on %s\n",
				ui.Render(theme.OK, "✓"),
				ui.Render(theme.BranchCurrent, res.Branch),
			)
			return nil
		},
	}
	register(cmd)
}

// stdinIsTTY reports whether standard input is connected to a
// character device. The picker needs raw-mode access to stdin, so we
// fall back to a static listing when it isn't.
func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// stdoutIsTTY reports whether standard output is a terminal. The
// picker draws to stdout, so a redirected stdout means we must use
// the static fallback to avoid emitting ANSI escape sequences into
// pipes.
func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
