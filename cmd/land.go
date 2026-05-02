package cmd

import (
	"errors"
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var (
		squash bool
		merge  bool
		rebase bool
		force  bool
	)
	cmd := &cobra.Command{
		Use:   "land",
		Short: "Merge the bottom-most PR in the stack and clean up locally",
		Long: "Merges the bottom-most tracked branch on the path from trunk to current via " +
			"`gh pr merge`, then runs `sm sync` so the merged branch is deleted locally and " +
			"its children retarget trunk. Refuses unless CI is green (use --force to skip).",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			method, err := pickMergeMethod(squash, merge, rebase)
			if err != nil {
				return err
			}
			r, err := newService().Land(c.Context(), service.LandOptions{
				Method: method,
				Force:  force,
			})
			if err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.OK, fmt.Sprintf("✓ merged #%d (%s) via %s", r.PR, r.Branch, r.Method)))
			if r.URL != "" {
				fmt.Println(ui.Render(theme.Dimmed, "  "+r.URL))
			}
			if r.Synced {
				if len(r.Sync.MergedBranches) > 0 {
					fmt.Println(ui.Render(theme.Dimmed, fmt.Sprintf("  cleaned up %d local branch(es)", len(r.Sync.MergedBranches))))
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&squash, "squash", false, "merge with squash strategy (default)")
	cmd.Flags().BoolVar(&merge, "merge", false, "merge with a merge commit")
	cmd.Flags().BoolVar(&rebase, "rebase", false, "merge with a rebase strategy")
	cmd.Flags().BoolVar(&force, "force", false, "skip the CI-green gate")
	register(cmd)
}

// pickMergeMethod converts the trio of mutually-exclusive flags into a
// single method value. Defaults to squash. Returns an error if more
// than one flag is set.
func pickMergeMethod(squash, merge, rebase bool) (gh.MergeMethod, error) {
	count := 0
	if squash {
		count++
	}
	if merge {
		count++
	}
	if rebase {
		count++
	}
	if count > 1 {
		return "", errors.New("--squash, --merge, and --rebase are mutually exclusive")
	}
	switch {
	case merge:
		return gh.MergeCommit, nil
	case rebase:
		return gh.MergeRebase, nil
	default:
		return gh.MergeSquash, nil
	}
}
