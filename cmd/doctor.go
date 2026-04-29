package cmd

import (
	"fmt"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "doctor",
		Aliases: []string{"status"},
		Short:   "Sanity-check stac-man metadata vs. git state",
		Args:    cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			r, err := newService().Doctor(c.Context())
			if err != nil {
				return err
			}
			renderDoctor(r)
			return nil
		},
	}
	register(cmd)
}

func renderDoctor(r service.DoctorReport) {
	fmt.Println(ui.Banner("stac-man doctor"))
	fmt.Println()
	fmt.Printf("%s %s (trunk)\n", ui.Render(theme.Header, "→"), ui.Render(theme.Accent, r.Trunk))
	fmt.Printf("%s %d tracked branches\n", ui.Render(theme.Header, "→"), r.TrackedCount)

	if len(r.NeedsRestack) == 0 && len(r.StaleSHA) == 0 && len(r.DriftedParent) == 0 && len(r.UntrackedRoots) == 0 && len(r.Issues) == 0 && len(r.MergeConflicts) == 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.OK, "✓ everything looks healthy"))
		return
	}

	if len(r.MergeConflicts) > 0 {
		// Surfaced first because a CONFLICTING PR will block any
		// `sm land` further down the stack and is the most likely
		// thing the user came to doctor to discover.
		fmt.Println()
		fmt.Println(ui.Render(theme.Fail, "PRs with merge conflicts:"))
		for _, mc := range r.MergeConflicts {
			label := mc.Branch
			if mc.PR > 0 {
				label = fmt.Sprintf("#%d %s", mc.PR, mc.Branch)
			}
			fmt.Println("  " + ui.Render(theme.MergeConflict, "⚠ "+label))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → resolve on GitHub or rebase locally; `sm sync` after an upstream merge often fixes this."))
	}

	if len(r.Issues) > 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Warn, "graph issues:"))
		for _, msg := range r.Issues {
			fmt.Println("  " + ui.Render(theme.Dimmed, "• "+msg))
		}
	}
	if len(r.NeedsRestack) > 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Warn, "needs restack:"))
		for _, b := range r.NeedsRestack {
			fmt.Println("  " + ui.Render(theme.BranchNeedsRestack, b))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → run `sm restack`"))
	}
	if len(r.StaleSHA) > 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Warn, "stale parent SHAs:"))
		for _, b := range r.StaleSHA {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → run `sm restack` to refresh"))
	}
	if len(r.DriftedParent) > 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Warn, "drifted parent SHAs:"))
		for _, b := range r.DriftedParent {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → branch history was rewritten outside `sm`. Run `sm restack` or fix manually."))
	}
	if len(r.UntrackedRoots) > 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Warn, "untracked branches with unique commits:"))
		for _, b := range r.UntrackedRoots {
			fmt.Println("  " + ui.Render(theme.Dimmed, b))
		}
		fmt.Println(ui.Render(theme.Dimmed, "  → checkout each and run `sm track` if you want it in the stack"))
	}
}
