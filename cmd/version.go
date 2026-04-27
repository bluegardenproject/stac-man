package cmd

import (
	"fmt"
	"runtime"

	"github.com/philipptpunkt/stac-man/internal/config"
	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version, build, and platform info",
		Long:  "Print the embedded version, build timestamp, Go runtime, platform, and config path.",
		Args:  cobra.NoArgs,
		Run: func(c *cobra.Command, args []string) {
			fmt.Printf("%s %s\n",
				ui.Render(theme.Title, "stac-man"),
				ui.Render(theme.Accent, Version),
			)
			fmt.Printf("  %s %s\n", ui.Render(theme.Header, "Built:"), ui.Render(theme.Info, BuildTime))
			fmt.Printf("  %s %s\n", ui.Render(theme.Header, "Go:"), ui.Render(theme.Info, runtime.Version()))
			fmt.Printf("  %s %s/%s\n",
				ui.Render(theme.Header, "Platform:"),
				ui.Render(theme.Info, runtime.GOOS),
				ui.Render(theme.Info, runtime.GOARCH),
			)
			cfgPath, err := config.Path()
			if err != nil {
				cfgPath = "(unavailable)"
			}
			fmt.Printf("  %s %s\n", ui.Render(theme.Header, "Config:"), ui.Render(theme.Info, cfgPath))
		},
	}
	register(cmd)
}
