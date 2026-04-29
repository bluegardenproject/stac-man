package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var (
		noPRStatus     bool
		noChecks       bool
		noMergeStatus  bool
	)
	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"ls"},
		Short:   "Print the stack tree",
		Long: "Render every tracked branch as a tree rooted at the trunk, decorated with " +
			"current-branch / needs-restack markers, PR status, a CI rollup dot, and a " +
			"GitHub mergeability glyph (✓ / ⚠ / ?). Pass --no-pr to skip the gh PR lookup, " +
			"--no-checks to drop the CI dot, or --no-merge-status to drop the mergeability " +
			"glyph. Statuses are cached for 60s under .git/stac-man/checks-cache.json.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			out, err := newService().Log(c.Context(), service.LogOptions{
				IncludePRStatus:    !noPRStatus,
				IncludeChecks:      !noPRStatus && !noChecks,
				IncludeMergeStatus: !noPRStatus && !noMergeStatus,
			})
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noPRStatus, "no-pr", false, "skip PR status lookup (no gh calls)")
	cmd.Flags().BoolVar(&noChecks, "no-checks", false, "skip the CI rollup dot per row")
	cmd.Flags().BoolVar(&noMergeStatus, "no-merge-status", false, "skip the GitHub mergeability glyph per row")

	register(cmd)
}
