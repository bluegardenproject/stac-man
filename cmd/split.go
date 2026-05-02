package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	var (
		namesFlag   string
		commitsFlag string
	)
	cmd := &cobra.Command{
		Use:   "split",
		Short: "Split the current branch into a chain of smaller branches",
		Long: "By default, splits the current branch into one branch per commit using the " +
			"slugified subject as the branch name. Pass --names a,b,c and --commits 1-2,3,4-5 " +
			"to drive an explicit grouping (commit indices are 1-based, oldest first).",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			s := newService()
			mappings, err := buildMappingsFromFlags(c.Context(), s, namesFlag, commitsFlag)
			if err != nil {
				return err
			}
			r, err := s.Split(c.Context(), service.SplitOptions{Mappings: mappings})
			if err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.OK, fmt.Sprintf("✓ split %s into %d branch(es):", r.Original, len(r.Created))))
			for _, name := range r.Created {
				fmt.Println("  " + ui.Render(theme.Accent, name))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&namesFlag, "names", "", "comma-separated branch names (paired with --commits)")
	cmd.Flags().StringVar(&commitsFlag, "commits", "", "comma-separated commit ranges per branch, e.g. 1-2,3,4-5")
	register(cmd)
}

// buildMappingsFromFlags combines --names and --commits into a slice
// of SplitMapping. Both flags must be provided together; if neither is
// set we return nil so Split falls back to its auto mode.
func buildMappingsFromFlags(ctx context.Context, s *service.Service, namesFlag, commitsFlag string) ([]service.SplitMapping, error) {
	if namesFlag == "" && commitsFlag == "" {
		return nil, nil
	}
	if namesFlag == "" || commitsFlag == "" {
		return nil, errors.New("--names and --commits must be used together")
	}
	names := splitCSV(namesFlag)
	groups := splitCSV(commitsFlag)
	if len(names) != len(groups) {
		return nil, fmt.Errorf("--names has %d entries but --commits has %d", len(names), len(groups))
	}

	commits, _, err := s.CommitsForCurrent(ctx)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, errors.New("current branch has no commits to split")
	}

	out := make([]service.SplitMapping, 0, len(names))
	for i, name := range names {
		idxs, err := service.ParseRanges(groups[i], len(commits))
		if err != nil {
			return nil, fmt.Errorf("commits[%d]: %w", i, err)
		}
		shas := make([]string, 0, len(idxs))
		for _, idx := range idxs {
			shas = append(shas, commits[idx].SHA)
		}
		out = append(out, service.SplitMapping{Name: strings.TrimSpace(name), Commits: shas})
	}
	return out, nil
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
