package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var (
		stack bool
		draft bool
		body  string
	)
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Push branches and open/update PRs via gh",
		Long: "Pushes the current branch (or, with --stack, the current branch and every " +
			"descendant) to origin and opens or updates pull requests via the gh CLI. " +
			"Existing PRs are retargeted when their base branch has changed locally.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			r, err := newService().Submit(c.Context(), service.SubmitOptions{
				Stack: stack,
				Draft: draft,
				Body:  body,
			})
			renderSubmitReport(r)
			return err
		},
	}
	cmd.Flags().BoolVar(&stack, "stack", false, "submit the current branch and every descendant")
	cmd.Flags().BoolVar(&draft, "draft", false, "create new PRs as drafts (existing PRs unchanged)")
	cmd.Flags().StringVar(&body, "body", "", "PR body for newly-created PRs")

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
	if len(r.Skipped) > 0 {
		fmt.Println(ui.Render(theme.Warn, "skipped:"))
		for _, s := range r.Skipped {
			fmt.Printf("  %s — %s\n", s.Branch, ui.Render(theme.Dimmed, s.Reason))
		}
	}
}
