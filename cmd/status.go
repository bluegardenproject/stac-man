package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/gh"
	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Fetch live GitHub status for tracked PRs",
		Long: "Fetch live GitHub checks and mergeability for every tracked branch with a recorded PR. " +
			"The result is written to the local cache so fast views can reuse the last known status.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			r, err := newService().Status(c.Context())
			if err != nil {
				return err
			}
			renderStatus(r)
			return nil
		},
	}
	register(cmd)
}

func renderStatus(r service.StatusReport) {
	fmt.Println(ui.Banner("stac-man status"))
	fmt.Println()
	fmt.Printf("%s %s (trunk)\n", ui.Render(theme.Header, "→"), ui.Render(theme.Accent, r.Trunk))
	if len(r.Branches) == 0 {
		fmt.Println()
		fmt.Println(ui.Render(theme.Dimmed, "  no tracked branches have recorded PRs yet"))
		return
	}
	fmt.Println()
	for _, b := range r.Branches {
		label := fmt.Sprintf("#%d %s", b.PR, b.Branch)
		if b.Err != "" {
			fmt.Printf("  %s %s %s\n", ui.Render(theme.Fail, "error"), label, ui.Render(theme.Dimmed, b.Err))
			continue
		}
		fmt.Printf("  %s %s", renderStatusState(b), label)
		if checks := renderStatusChecks(b.Checks); checks != "" {
			fmt.Printf(" %s", checks)
		}
		if merge := renderStatusMerge(b.Mergeable); merge != "" {
			fmt.Printf(" %s", merge)
		}
		fmt.Println()
	}
}

func renderStatusState(b service.StatusBranch) string {
	if b.Draft {
		return ui.Render(theme.PRDraft, "draft")
	}
	switch b.State {
	case gh.PRStateMerged:
		return ui.Render(theme.PRMerged, "merged")
	case gh.PRStateClosed:
		return ui.Render(theme.PRClosed, "closed")
	default:
		return ui.Render(theme.PROpen, "open")
	}
}

func renderStatusChecks(c gh.CheckRollup) string {
	switch c {
	case gh.ChecksPass:
		return ui.Render(theme.BadgeCIPass, "CI pass")
	case gh.ChecksPending:
		return ui.Render(theme.BadgeCIPending, "CI pending")
	case gh.ChecksFail:
		return ui.Render(theme.BadgeCIFail, "CI fail")
	default:
		return ""
	}
}

func renderStatusMerge(m gh.Mergeability) string {
	switch m {
	case gh.MergeMergeable:
		return ui.Render(theme.BadgeMergeReady, "mergeable")
	case gh.MergeConflicting:
		return ui.Render(theme.BadgeMergeConflict, "conflict")
	default:
		return ""
	}
}
