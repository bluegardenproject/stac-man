package cmd

import (
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "undo",
		Short: "Roll back the most recent stac-man operation",
		Long: "Pops the latest entry from .git/stac-man/history.json and restores every " +
			"branch it touched (tip, parent metadata, tracking). With --dry-run, prints what " +
			"would be reversed without changing anything.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			r, err := newService().Undo(c.Context(), dryRun)
			if err != nil {
				return err
			}
			verb := "✓ undid"
			if r.DryRun {
				verb = "would undo"
			}
			line := fmt.Sprintf("%s %s", verb, r.Op)
			if r.Notes != "" {
				line += " (" + r.Notes + ")"
			}
			fmt.Println(ui.Render(theme.OK, line))
			if len(r.Branches) > 0 {
				fmt.Println(ui.Render(theme.Dimmed, "  branches: "+strings.Join(r.Branches, ", ")))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be reversed without making changes")
	register(cmd)
}
