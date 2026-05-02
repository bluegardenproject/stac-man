package cmd

import (
	"fmt"
	"os"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/progress"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
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
			report, err := newService().Sync(c.Context(), service.SyncOptions{
				Progress: progress.New(os.Stdout),
			})
			renderSyncReport(report)
			return printRestackOutcome(err)
		},
	}
	register(cmd)
}

func renderSyncReport(r service.SyncReport) {
	// Trunk fetch + pull is now reported via the progress reporter
	// (one ✓-line per phase), so the static "pulled <trunk>" line
	// here would just duplicate it. Worse, the old version printed
	// unconditionally even when the pull never ran (e.g. dirty
	// tree errored out before the fetch). The summary now sticks
	// to information that's exclusive to the report — merged
	// branches and retargeted PRs.
	if len(r.MergedBranches) > 0 {
		fmt.Println(ui.Render(theme.Header, "merged & deleted:"))
		for _, b := range r.MergedBranches {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
	}
	if len(r.RetargetedPRs) > 0 {
		fmt.Println(ui.Render(theme.Header, "retargeted PR bases on GitHub:"))
		for _, rp := range r.RetargetedPRs {
			line := fmt.Sprintf("  #%d (%s) → %s", rp.PR, rp.Branch, rp.NewBase)
			if rp.Err != "" {
				fmt.Printf("  %s %s — %s\n",
					ui.Render(theme.Warn, "!"),
					line[2:],
					ui.Render(theme.Dimmed, rp.Err),
				)
				continue
			}
			fmt.Println(ui.Render(theme.Accent, line))
		}
	}
}
