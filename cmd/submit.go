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
	var (
		stack        bool
		draft        bool
		body         string
		noRestack    bool
		noStackTable bool
	)
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Push branches and open/update PRs via gh",
		Long: "Pushes the current branch (or, with --stack, the current branch and every " +
			"descendant) to origin and opens or updates pull requests via the gh CLI. " +
			"Existing PRs are retargeted when their base branch has changed locally. " +
			"Each PR body gets a sentinel-fenced \"Stack\" block at the top showing " +
			"every PR in the chain and the reviewer's position in it; pass --no-stack-table " +
			"to opt out. Branches whose recorded parent SHA is stale are skipped with a " +
			"\"needs restack\" message by default; pass --no-restack to push them anyway " +
			"and accept the noisy diff.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			r, err := newService().Submit(c.Context(), service.SubmitOptions{
				Stack:        stack,
				Draft:        draft,
				Body:         body,
				NoRestack:    noRestack,
				NoStackTable: noStackTable,
				Progress:     progress.New(os.Stdout),
			})
			renderSubmitReport(r)
			return err
		},
	}
	cmd.Flags().BoolVar(&stack, "stack", false, "submit every ancestor, the current branch, and every descendant (idempotent: branches already in sync with origin are not re-pushed)")
	cmd.Flags().BoolVar(&draft, "draft", false, "create new PRs as drafts (existing PRs unchanged)")
	cmd.Flags().StringVar(&body, "body", "", "PR body for newly-created PRs")
	cmd.Flags().BoolVar(&noRestack, "no-restack", false, "push branches even when their recorded parent SHA is stale (warn instead of skip)")
	cmd.Flags().BoolVar(&noStackTable, "no-stack-table", false, "do not inject the auto-generated stack table into PR bodies")

	register(cmd)
}

func renderSubmitReport(r service.SubmitReport) {
	if len(r.Pushed) > 0 {
		fmt.Println(ui.Render(theme.Header, "pushed:"))
		for _, b := range r.Pushed {
			fmt.Println("  " + ui.Render(theme.Accent, b))
		}
	}
	if len(r.Created) > 0 {
		fmt.Println(ui.Render(theme.Header, "created PRs:"))
		for _, p := range r.Created {
			fmt.Printf("  %s %s\n", ui.Render(theme.PROpen, fmt.Sprintf("#%d", p.Number)), p.Branch)
		}
	}
	if len(r.Updated) > 0 {
		fmt.Println(ui.Render(theme.Header, "updated PRs:"))
		for _, p := range r.Updated {
			fmt.Printf("  %s %s\n", ui.Render(theme.PROpen, fmt.Sprintf("#%d", p.Number)), p.Branch)
		}
	}
	if len(r.SkippedPushes) > 0 {
		fmt.Println(ui.Render(theme.Header, "already in sync with origin:"))
		for _, b := range r.SkippedPushes {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
	}
	if len(r.StaleParentSHA) > 0 {
		fmt.Println(ui.Render(theme.Warn, "pushed with stale parent SHA (--no-restack):"))
		for _, b := range r.StaleParentSHA {
			fmt.Println("  " + ui.Render(theme.BranchNeedsRestack, b))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → diff on GitHub may include parent commits; run `sm restack` then `sm submit` to clean up."))
	}
	if len(r.Skipped) > 0 {
		fmt.Println(ui.Render(theme.Warn, "skipped:"))
		for _, s := range r.Skipped {
			fmt.Printf("  %s — %s\n", s.Branch, ui.Render(theme.Dimmed, s.Reason))
		}
	}
	if len(r.DivergedStackmates) > 0 {
		fmt.Println(ui.Render(theme.Warn, "stack-mates diverged from origin:"))
		for _, d := range r.DivergedStackmates {
			label := d.Branch
			if d.PR > 0 {
				label = fmt.Sprintf("#%d %s", d.PR, d.Branch)
			}
			fmt.Println("  " + ui.Render(theme.Dimmed, label))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → run `sm submit --stack` from the bottom to refresh"))
	}
}
