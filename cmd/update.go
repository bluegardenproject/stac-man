package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/bluegardenproject/stac-man/internal/update"
	"github.com/spf13/cobra"
)

var flagUpdateCheck bool

func init() {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Install the latest released version of stac-man",
		Long: `Update stac-man to the latest release.

  sm update           install the latest version
  sm update --check   compare the running binary's version against the latest release without installing

Updates are performed by re-running the install script for your
platform, which downloads the binary into ~/.stac-man/.

Dev builds (those reporting "dev") cannot self-update — install a
tagged release first via the curl one-liner in the README.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			ctx := c.Context()

			if Version == "dev" || Version == "unknown" {
				fmt.Println(ui.Render(theme.Warn,
					"dev build — install a release to enable self-updates"))
				fmt.Println("  see the README for the curl install one-liner.")
				return nil
			}

			rel, err := update.LatestRelease(ctx)
			if err != nil {
				return fmt.Errorf("check for updates: %w", err)
			}

			cmp := update.Compare(Version, rel.TagName)
			fmt.Printf("%s %s\n",
				ui.Render(theme.Header, "Current:"),
				ui.Render(theme.Info, Version),
			)
			fmt.Printf("%s  %s\n",
				ui.Render(theme.Header, "Latest:"),
				ui.Render(theme.Info, rel.TagName),
			)

			if cmp >= 0 {
				fmt.Println(ui.Render(theme.OK, "You're already on the latest version."))
				return nil
			}

			if flagUpdateCheck {
				fmt.Println(ui.Render(theme.Warn,
					fmt.Sprintf("A newer version is available: %s", rel.TagName)))
				fmt.Println("  run `sm update` to install it.")
				return nil
			}

			fmt.Println(ui.Render(theme.Accent, "Installing latest version..."))
			if err := update.Run(ctx); err != nil {
				return fmt.Errorf("run installer: %w", err)
			}
			fmt.Println(ui.Render(theme.OK, "Update complete."))
			return nil
		},
	}

	cmd.Flags().BoolVar(&flagUpdateCheck, "check", false,
		"only report whether an update is available; do not install")

	register(cmd)
}
