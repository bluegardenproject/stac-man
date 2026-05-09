package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bluegardenproject/stac-man/internal/service"
	"github.com/bluegardenproject/stac-man/internal/ui/progress"
	"github.com/spf13/cobra"
)

func init() {
	var (
		noPRStatus   bool
		jsonOut      bool
		porcelainOut bool
	)
	cmd := &cobra.Command{
		Use:     "log",
		Aliases: []string{"ls"},
		Short:   "Print the stack tree",
		Long: "Render every tracked branch as a tree rooted at the trunk, decorated with " +
			"current-branch / needs-restack markers and cached PR summaries. Pass --no-pr " +
			"to skip cached PR decoration entirely. Live GitHub checks and mergeability " +
			"live under `sm status` so `sm log` stays fast and local-first.\n\n" +
			"For scripts and AI agents, --json emits the same graph as a single JSON " +
			"document (per-branch shape matches `sm show --json`) and --porcelain emits a " +
			"stable tab-separated row per branch with no header.",
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if jsonOut && porcelainOut {
				return fmt.Errorf("--json and --porcelain are mutually exclusive")
			}
			opts := service.LogOptions{
				IncludePRStatus:    !noPRStatus,
				IncludeChecks:      false,
				IncludeMergeStatus: false,
			}
			svc := newService()
			if jsonOut || porcelainOut {
				// Machine-readable output: any spinner / status line
				// would corrupt the stream consumers parse.
				opts.Progress = progress.Discard()
				result, err := svc.LogData(c.Context(), opts)
				if err != nil {
					return err
				}
				if jsonOut {
					encoded, err := json.MarshalIndent(result, "", "  ")
					if err != nil {
						return err
					}
					fmt.Println(string(encoded))
					return nil
				}
				fmt.Print(service.FormatLogPorcelain(result))
				return nil
			}
			opts.Progress = progress.New(os.Stdout)
			out, err := svc.Log(c.Context(), opts)
			if err != nil {
				return err
			}
			fmt.Print(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&noPRStatus, "no-pr", false, "skip cached PR decoration")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the stack as JSON (per-branch shape matches `sm show --json`)")
	cmd.Flags().BoolVar(&porcelainOut, "porcelain", false, "emit one tab-separated row per branch with no header")

	register(cmd)
}
