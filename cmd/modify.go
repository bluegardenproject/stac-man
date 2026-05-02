package cmd

import (
	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/spf13/cobra"
)

func init() {
	var (
		commit           bool
		amend            bool
		stage            bool
		includeUntracked bool
		message          string
	)
	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Amend the current branch and restack descendants",
		Long: "modify is the everyday `commit + restack` shortcut. With -c it creates a " +
			"new commit; with --amend it replaces the current commit. -a stages " +
			"tracked-but-modified files (matching `git commit -a`); pass --include-untracked " +
			"to also include new files. After the history change every tracked descendant " +
			"is rebased so they pick up the new tip.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			// `--amend` defaults to true, but if the user passed `-c`
			// without explicitly passing `--amend`, treat amend as off
			// so the two flags don't fight each other.
			amendExplicit := c.Flags().Changed("amend")
			effectiveAmend := amend
			if commit && !amendExplicit {
				effectiveAmend = false
			}
			err := newService().Modify(c.Context(), service.ModifyOptions{
				Commit:           commit,
				Amend:            effectiveAmend,
				StageAll:         stage,
				IncludeUntracked: includeUntracked,
				Message:          message,
			})
			return printRestackOutcome(err)
		},
	}
	cmd.Flags().BoolVarP(&commit, "commit", "c", false, "create a new commit instead of amending")
	cmd.Flags().BoolVar(&amend, "amend", true, "amend the current commit (default)")
	cmd.Flags().BoolVarP(&stage, "all", "a", false, "stage tracked-but-modified files before committing (mirrors `git commit -a`)")
	cmd.Flags().BoolVar(&includeUntracked, "include-untracked", false, "with -a, also stage new (untracked) files")
	cmd.Flags().StringVarP(&message, "message", "m", "", "commit message (required with -c)")

	register(cmd)
}
