package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var reparent bool
	cmd := &cobra.Command{
		Use:   "untrack [branch]",
		Short: "Remove a branch from the stack graph",
		Long: "Untrack drops stac-man's metadata for a branch but does not delete the " +
			"branch itself. If the branch has tracked children pass --reparent to " +
			"move them onto its parent.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			if err := newService().Untrack(c.Context(), branch, reparent); err != nil {
				return err
			}
			displayBranch := branch
			if displayBranch == "" {
				displayBranch = "current branch"
			}
			fmt.Printf("%s untracked %s\n",
				ui.Render(theme.OK, "✓"),
				ui.Render(theme.Accent, displayBranch),
			)
			return nil
		},
	}
	cmd.Flags().BoolVar(&reparent, "reparent", false, "re-parent tracked children onto the untracked branch's parent")

	register(cmd)
}
