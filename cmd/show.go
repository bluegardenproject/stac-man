package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/philipptpunkt/stac-man/internal/service"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show [branch]",
		Short: "Show a branch's parent, children, PR status, and commits",
		Long: "Prints a detailed view of a single branch: its position in the stack, ahead/" +
			"behind counts vs parent and trunk, PR state, and the commits unique to it. " +
			"With --json, emits a machine-readable BranchView for scripts and AI agents.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: branchNameCompletion,
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			view, err := newService().Show(c.Context(), branch)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(view)
			}
			fmt.Print(renderBranchView(view))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit a JSON BranchView instead of a human-readable summary")
	register(cmd)
}

// renderBranchView formats the view for terminal output, using the
// shared theme so it matches `sm log`.
func renderBranchView(v *service.BranchView) string {
	var b strings.Builder
	b.WriteString(ui.Banner(v.Branch) + "\n\n")

	row := func(label, value string) {
		if value == "" {
			return
		}
		b.WriteString("  " + ui.Render(theme.Header, label) + " " + value + "\n")
	}
	dim := func(s string) string { return ui.Render(theme.Dimmed, s) }

	if v.Tracked {
		parent := v.Parent
		if parent == "" {
			parent = v.Trunk
		}
		row("Parent:", ui.Render(theme.Accent, parent))
		if v.NeedsRestack {
			row("Status:", ui.Render(theme.Warn, "needs restack"))
		} else {
			row("Status:", ui.Render(theme.OK, "up to date"))
		}
	} else {
		row("Status:", dim("untracked — `sm track` to adopt"))
	}
	row("Trunk:", ui.Render(theme.BranchTrunk, v.Trunk))
	if v.Tip != "" {
		row("Tip:", dim(short(v.Tip)))
	}

	parent := v.Parent
	if parent == "" {
		parent = v.Trunk
	}
	row("vs "+parent+":", fmt.Sprintf("%s %s",
		ui.Render(theme.OK, fmt.Sprintf("+%d", v.AheadParent)),
		ui.Render(theme.Warn, fmt.Sprintf("-%d", v.BehindParent)),
	))
	if parent != v.Trunk {
		row("vs "+v.Trunk+":", fmt.Sprintf("%s %s",
			ui.Render(theme.OK, fmt.Sprintf("+%d", v.AheadTrunk)),
			ui.Render(theme.Warn, fmt.Sprintf("-%d", v.BehindTrunk)),
		))
	}

	if len(v.Ancestors) > 0 {
		row("Ancestors:", strings.Join(v.Ancestors, " → "))
	}
	if len(v.Children) > 0 {
		row("Children:", strings.Join(v.Children, ", "))
	}

	if v.PR != nil {
		state := v.PR.State
		stateStyle := theme.PROpen
		switch state {
		case "MERGED":
			stateStyle = theme.PRMerged
		case "CLOSED":
			stateStyle = theme.PRClosed
		}
		if v.PR.Draft {
			stateStyle = theme.PRDraft
			state = "DRAFT"
		}
		row("PR:", fmt.Sprintf("%s %s %s",
			ui.Render(theme.Accent, fmt.Sprintf("#%d", v.PR.Number)),
			ui.Render(stateStyle, state),
			dim(v.PR.URL),
		))
		if v.PR.Title != "" {
			row("Title:", v.PR.Title)
		}
	}

	if len(v.Commits) > 0 {
		b.WriteString("\n  " + ui.Render(theme.Header, "Commits:") + "\n")
		for i, c := range v.Commits {
			b.WriteString(fmt.Sprintf("    %s %s %s\n",
				dim(fmt.Sprintf("%2d.", i+1)),
				ui.Render(theme.Accent, short(c.SHA)),
				c.Subject,
			))
		}
	}
	return b.String()
}
