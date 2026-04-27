package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var noPRStatus bool
	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"ls"},
		Short:   "Print the stack tree",
		Long: "Render every tracked branch as a tree rooted at the trunk, decorated with " +
			"current-branch / needs-restack markers and PR status. Pass --no-pr to skip " +
			"the gh PR lookup.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			out, err := newService().Log(c.Context(), service.LogOptions{
				IncludePRStatus: !noPRStatus,
			})
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noPRStatus, "no-pr", false, "skip PR status lookup (no gh calls)")

	register(cmd)
}
