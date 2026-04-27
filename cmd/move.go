package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	var onto string
	cmd := &cobra.Command{
		Use:   "move [branch]",
		Short: "Reparent a branch (and its subtree) onto a new base",
		Long: "Reassigns the parent of <branch> (default: current) to --onto and restacks " +
			"the whole subtree. Descendants ride along automatically. Equivalent to " +
			"`sm parent --set` but reads naturally for whole-stack moves.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: branchNameCompletion,
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			err := newService().Move(c.Context(), branch, onto)
			return printRestackOutcome(err)
		},
	}
	cmd.Flags().StringVar(&onto, "onto", "", "new parent branch (required)")
	_ = cmd.RegisterFlagCompletionFunc("onto", branchNameCompletion)
	_ = cmd.MarkFlagRequired("onto")
	register(cmd)
}
