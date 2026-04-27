package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var message string
	cmd := &cobra.Command{
		Use:   "fold",
		Short: "Squash the current branch into its parent",
		Long: "Squashes every commit unique to the current branch into a single commit " +
			"on its parent, deletes the (now-folded) branch, and re-parents any tracked " +
			"children. Use this once a stack entry's history has settled and you want a " +
			"clean linear parent.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if err := newService().Fold(c.Context(), message); err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.OK, "✓ folded into parent"))
			return nil
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message for the squashed commit (default: \"fold <branch> into <parent>\")")
	register(cmd)
}
