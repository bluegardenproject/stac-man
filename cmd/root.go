// Package cmd defines the cobra command tree for the `sm` binary.
//
// Each subcommand lives in its own file (create.go, modify.go, …) and is
// wired into the root command via init(). The root command itself only
// owns global flags and pre-run plumbing (color, version) so that
// subcommands stay focused on a single feature.
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

// Version is the binary version. Overridden at build time via -ldflags.
var Version = "0.0.0-dev"

// Global flags exposed on the root command. Subcommands read these via
// the package-level vars below rather than re-declaring them.
var (
	flagNoColor bool
	flagVerbose bool
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sm",
		Short:         "stac-man — a CLI for stacked pull requests",
		Long:          "stac-man (sm) manages stacked branches and the pull requests that go with them, using git config as the source of truth and the gh CLI for GitHub interactions.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if flagNoColor {
				_ = os.Setenv("NO_COLOR", "1")
			}
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable all color output (also respects NO_COLOR env var)")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output (prints underlying git/gh commands)")

	return root
}

// Execute runs the root command. Cancellation from ctx is passed through
// to subcommands via cobra's SetContext so a Ctrl+C tears down cleanly.
func Execute(ctx context.Context) error {
	root := newRootCmd()
	root.SetContext(ctx)
	return root.Execute()
}
