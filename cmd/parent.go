package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var setParent string
	cmd := &cobra.Command{
		Use:   "parent [branch]",
		Short: "Show or change the parent of a branch",
		Long: "Without --set, prints the parent of the named branch (or the current branch). " +
			"With --set <branch>, reassigns the parent and restacks history onto the new base.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			s := newService()
			if setParent != "" {
				err := s.SetParent(c.Context(), branch, setParent)
				return printRestackOutcome(err)
			}
			parent, err := s.Parent(c.Context(), branch)
			if err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.Accent, parent))
			return nil
		},
	}
	cmd.Flags().StringVar(&setParent, "set", "", "reassign parent to the given branch and restack")

	register(cmd)
}
