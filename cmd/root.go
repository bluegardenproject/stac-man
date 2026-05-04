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

	"github.com/bluegardenproject/stac-man/internal/tui/cockpit"
	"github.com/spf13/cobra"
)

// Version and BuildTime are set by main.SetVersion at process start.
// They live in package main so Release Please's `extra-files` config
// can rewrite the literals without touching cmd/.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// SetVersion is called from main() with the ldflags-injected values
// before the cobra tree runs. Keeping it a setter (rather than reading
// a package-main variable directly) lets cmd/ stay free of an import
// cycle on the parent module.
func SetVersion(version, buildTime string) {
	if version != "" {
		Version = version
	}
	if buildTime != "" {
		BuildTime = buildTime
	}
}

// Global flags exposed on the root command. Subcommands read these via
// the package-level vars below rather than re-declaring them.
var (
	flagNoColor bool
	flagVerbose bool
)

// pendingSubcommands is populated by each subcommand's init() via the
// register() helper. We attach them in newRootCmd so init order between
// files doesn't matter.
var pendingSubcommands []*cobra.Command

// register adds a subcommand to be wired into the root in newRootCmd.
// Files in this package call register(...) from init() to keep
// per-command wiring close to the command itself.
func register(cmd *cobra.Command) {
	pendingSubcommands = append(pendingSubcommands, cmd)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sm",
		Short:         "stac-man — a CLI for stacked pull requests",
		Long:          "stac-man (sm) manages stacked branches and the pull requests that go with them, using git config as the source of truth and the gh CLI for GitHub interactions.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// BREAKING (v1.0.0): bare `sm` on a TTY now opens the
		// interactive cockpit instead of printing help. Pipes, CI,
		// or any non-TTY context still get help so existing
		// scripts are unaffected.
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if !stdinIsTTY() || !stdoutIsTTY() {
				return c.Help()
			}
			return cockpit.Run(c.Context(), newService())
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if flagNoColor {
				_ = os.Setenv("NO_COLOR", "1")
			}
			return nil
		},
	}

	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable all color output (also respects NO_COLOR env var)")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output (prints underlying git/gh commands)")

	for _, sub := range pendingSubcommands {
		root.AddCommand(sub)
	}

	return root
}

// Execute runs the root command. Cancellation from ctx is passed through
// to subcommands via cobra's SetContext so a Ctrl+C tears down cleanly.
func Execute(ctx context.Context) error {
	root := newRootCmd()
	root.SetContext(ctx)
	return root.Execute()
}

// Root returns a freshly-built command tree without executing it. It's
// intended for tooling that needs to introspect the cobra surface
// (e.g. tools/docscan dumps it as JSON for the docs-drift-audit skill).
// Each call returns a new tree; callers should not assume identity.
func Root() *cobra.Command {
	return newRootCmd()
}
