package cmd

import (
	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var (
		commit  bool
		amend   bool
		stage   bool
		message string
	)
	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Amend the current branch and restack descendants",
		Long: "modify is the everyday `commit + restack` shortcut. With -c it creates a " +
			"new commit, with --amend it replaces the current commit, and -a stages all " +
			"unstaged changes first. After the history change every tracked descendant " +
			"is rebased so they pick up the new tip.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			err := newService().Modify(c.Context(), service.ModifyOptions{
				Commit:   commit,
				Amend:    amend,
				StageAll: stage,
				Message:  message,
			})
			return printRestackOutcome(err)
		},
	}
	cmd.Flags().BoolVarP(&commit, "commit", "c", false, "create a new commit instead of amending")
	cmd.Flags().BoolVar(&amend, "amend", true, "amend the current commit (default)")
	cmd.Flags().BoolVarP(&stage, "all", "a", false, "stage all unstaged changes before committing")
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message (required with -c)")

	register(cmd)
}
