package cmd

import (
	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var (
		message          string
		stageAll         bool
		includeUntracked bool
	)
	cmd := &cobra.Command{
		Use:   "create <branch>",
		Short: "Create a new branch on top of the current one",
		Long: "Create a new branch off HEAD and record the current branch as its parent. " +
			"With -m the new branch starts with a commit. -a stages tracked-but-modified " +
			"files (matching `git commit -a`); pass --include-untracked to also include new " +
			"files. New files without that flag must be staged via `git add` first.",
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return newService().Create(c.Context(), service.CreateOptions{
				Name:             args[0],
				CommitMessage:    message,
				StageAll:         stageAll,
				IncludeUntracked: includeUntracked,
			})
		},
	}
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message for an initial commit on the new branch")
	cmd.Flags().BoolVarP(&stageAll, "all", "a", false, "stage tracked-but-modified files before committing (mirrors `git commit -a`)")
	cmd.Flags().BoolVar(&includeUntracked, "include-untracked", false, "with -a, also stage new (untracked) files")

	register(cmd)
}
