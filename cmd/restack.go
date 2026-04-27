package cmd

import (
	"errors"
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "restack [branch]",
		Aliases: []string{"rs"},
		Short:   "Rebase the current branch and its descendants onto their parents",
		Long: "Walks the stack rooted at the current (or named) branch in topological " +
			"order, rebasing each branch onto its parent's tip. On conflict the walk " +
			"pauses; resolve the conflict and run `sm continue` (or `sm abort`).",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			err := newService().Restack(c.Context(), branch)
			return printRestackOutcome(err)
		},
	}
	register(cmd)

	register(&cobra.Command{
		Use:   "continue",
		Short: "Resume a paused restack/sync after resolving conflicts",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			err := newService().RestackContinue(c.Context())
			return printRestackOutcome(err)
		},
	})

	register(&cobra.Command{
		Use:   "abort",
		Short: "Abort a paused restack/sync and restore the original branch",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if err := newService().RestackAbort(c.Context()); err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.OK, "✓ aborted; original branch restored"))
			return nil
		},
	})
}

// printRestackOutcome formats success/pause/error for restack-flavored
// commands. Pauses are printed as a friendly multi-line guide rather
// than an ugly stack trace.
func printRestackOutcome(err error) error {
	if err == nil {
		fmt.Println(ui.Render(theme.OK, "✓ stack is up to date"))
		return nil
	}
	var paused *service.PausedError
	if errors.As(err, &paused) {
		fmt.Println(ui.Render(theme.Warn, "⚠ rebase paused on "+paused.Branch))
		fmt.Println(ui.Render(theme.Dimmed, "  resolve the conflicts, `git add` the resolved files, then run:"))
		fmt.Println(ui.Render(theme.Accent, "    sm continue"))
		fmt.Println(ui.Render(theme.Dimmed, "  or to bail out:"))
		fmt.Println(ui.Render(theme.Accent, "    sm abort"))
		return err
	}
	return err
}
