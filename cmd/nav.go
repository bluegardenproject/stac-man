package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

// navCommand is the shared factory for the four directional commands.
// They differ only in the Direction they pass into Service.Navigate.
func navCommand(use, short string, dir service.Direction) *cobra.Command {
	var first bool
	c := &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			target, err := newService().Navigate(c.Context(), dir, first)
			if err != nil {
				return err
			}
			fmt.Printf("%s now on %s\n",
				ui.Render(theme.OK, "✓"),
				ui.Render(theme.BranchCurrent, target),
			)
			return nil
		},
	}
	c.Flags().BoolVar(&first, "first", false, "at a fork, take the alphabetically first branch instead of erroring")
	return c
}

func init() {
	register(navCommand("up", "Move HEAD to the first child of the current branch", service.DirUp))
	register(navCommand("down", "Move HEAD to the parent of the current branch", service.DirDown))
	register(navCommand("top", "Walk up to a leaf of the current sub-stack", service.DirTop))
	register(navCommand("bottom", "Walk down to the branch sitting directly on trunk", service.DirBottom))
}
