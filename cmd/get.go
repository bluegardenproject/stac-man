package cmd

import (
	"fmt"
	"strconv"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "get <PR-number>",
		Short: "Fetch a colleague's stack locally and reproduce its parent edges",
		Long: "Walks the base_ref chain of <PR-number>, checks each branch out via " +
			"`gh pr checkout`, and records parent metadata so `sm log` mirrors the PR " +
			"author's stack. After it returns, HEAD is on the top branch.",
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			n, err := strconv.Atoi(args[0])
			if err != nil || n <= 0 {
				return fmt.Errorf("invalid PR number %q", args[0])
			}
			s := newService()
			r, err := s.Get(c.Context(), n)
			if err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.OK, fmt.Sprintf("✓ pulled %d branch(es) from #%d", len(r.Branches), n)))
			out, lerr := s.Log(c.Context(), service.LogOptions{IncludePRStatus: true})
			if lerr == nil {
				fmt.Print(out)
			}
			return nil
		},
	}
	register(cmd)
}
