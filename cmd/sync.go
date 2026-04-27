package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Pull trunk, delete merged branches, restack survivors",
		Long: "Fetches origin, fast-forwards the local trunk, deletes any tracked branch " +
			"whose commits are fully merged into trunk, re-parents their children onto the " +
			"merged branch's parent, and restacks every surviving root.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			report, err := newService().Sync(c.Context())
			renderSyncReport(report)
			return printRestackOutcome(err)
		},
	}
	register(cmd)
}

func renderSyncReport(r service.SyncReport) {
	fmt.Printf("%s pulled %s\n",
		ui.Render(theme.OK, "✓"),
		ui.Render(theme.Accent, r.Trunk),
	)
	if len(r.MergedBranches) > 0 {
		fmt.Println(ui.Render(theme.Header, "merged & deleted:"))
		for _, b := range r.MergedBranches {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
	}
}
