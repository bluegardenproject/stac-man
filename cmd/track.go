package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var parent string
	cmd := &cobra.Command{
		Use:   "track [branch]",
		Short: "Adopt an existing branch into the stack graph",
		Long: "Track records the parent of a branch in stac-man's metadata so subsequent " +
			"restack/sync commands include it. With no argument the current branch is " +
			"tracked; with --parent the parent is set explicitly, otherwise it's inferred.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			parentName, err := newService().Track(c.Context(), service.TrackOptions{
				Branch: branch,
				Parent: parent,
			})
			if err != nil {
				return err
			}
			displayBranch := branch
			if displayBranch == "" {
				displayBranch = "current branch"
			}
			fmt.Printf("%s tracked %s onto parent %s\n",
				ui.Render(theme.OK, "✓"),
				ui.Render(theme.Accent, displayBranch),
				ui.Render(theme.Info, parentName),
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&parent, "parent", "", "explicit parent branch (default: inferred from merge-base)")

	register(cmd)
}
