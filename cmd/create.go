package cmd

import (
	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var (
		message  string
		stageAll bool
	)
	cmd := &cobra.Command{
		Use:   "create <branch>",
		Short: "Create a new branch on top of the current one",
		Long: "Create a new branch off HEAD and record the current branch as its parent. " +
			"With -m the new branch starts with a commit; with -a all unstaged changes are " +
			"included in that commit.",
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return newService().Create(c.Context(), service.CreateOptions{
				Name:          args[0],
				CommitMessage: message,
				StageAll:      stageAll,
			})
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message for an initial commit on the new branch")
	cmd.Flags().BoolVarP(&stageAll, "all", "a", false, "stage all changes before committing")

	register(cmd)
}
