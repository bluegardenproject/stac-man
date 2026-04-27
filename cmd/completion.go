package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Generate shell completion scripts",
		Long: `Generate the autocompletion script for sm in the named shell. Source the
output to enable tab-completion of subcommands and tracked branch names.

Bash:
  $ sm completion bash > /etc/bash_completion.d/sm

Zsh:
  $ sm completion zsh > "${fpath[1]}/_sm"
  # then restart your shell

Fish:
  $ sm completion fish > ~/.config/fish/completions/sm.fish

PowerShell:
  PS> sm completion powershell > sm.ps1
  PS> . ./sm.ps1
`,
		Args:                  cobra.ExactValidArgs(1),
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		DisableFlagsInUseLine: true,
		RunE: func(c *cobra.Command, args []string) error {
			root := c.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	register(cmd)
}

// branchNameCompletion returns a cobra ValidArgsFunction that completes
// branch arguments with the list of tracked branches plus the trunk.
// Used by checkout, parent, move, and show.
func branchNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		// Single-arg commands: don't suggest more after the first arg.
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	s := newService()
	if err := s.EnsureRepo(ctx); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	tracked, err := s.Store.ListTrackedBranches(ctx)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	trunk, _ := s.EnsureTrunk(ctx)
	all := tracked
	if trunk != "" {
		all = append([]string{trunk}, tracked...)
	}
	return all, cobra.ShellCompDirectiveNoFileComp
}
