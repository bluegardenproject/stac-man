package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var base string
	cmd := &cobra.Command{
		Use:   "absorb",
		Short: "Auto-route uncommitted hunks into the right ancestor commits",
		Long: "Wraps `git-absorb` to fix up the right ancestor commits with your " +
			"current uncommitted changes, then restacks all descendants. Requires the " +
			"`git-absorb` binary on PATH (https://github.com/tummychow/git-absorb).",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			res, err := newService().Absorb(c.Context(), service.AbsorbOptions{Base: base})
			if err != nil {
				return printRestackOutcome(err)
			}
			fmt.Println(ui.Render(theme.OK, fmt.Sprintf("✓ absorbed into ancestors of %s (base: %s)", res.Branch, res.Base)))
			if res.Restacked {
				fmt.Println(ui.Render(theme.Dimmed, "  descendants restacked"))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&base, "base", "", "use this branch as the absorb base (default: lowest tracked ancestor)")
	register(cmd)
}
