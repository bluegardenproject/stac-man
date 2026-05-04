package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/bluegardenproject/stac-man/internal/config"
	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect or edit the user config (~/.config/stac-man/config.yaml)",
		Long: `Inspect or edit the optional user config.

  sm config path     print the absolute path of config.yaml
  sm config show     print the effective config (defaults + overrides)
  sm config edit     open config.yaml in $EDITOR (creates defaults first)
  sm config init     write a default config.yaml if one doesn't exist

The file is optional. Stack metadata lives in each repo's local git
config and is unaffected by this command. Updating sm via 'sm update'
never overwrites this file.`,
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error { return c.Help() },
	}

	cmd.AddCommand(configPathCmd())
	cmd.AddCommand(configShowCmd())
	cmd.AddCommand(configEditCmd())
	cmd.AddCommand(configInitCmd())

	register(cmd)
}

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the path to config.yaml",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Println(p)
			return nil
		},
	}
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective config",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				return err
			}
			p, _ := config.Path()
			if _, statErr := os.Stat(p); os.IsNotExist(statErr) {
				fmt.Println(ui.Render(theme.Dimmed,
					"# (no config file at "+p+" — showing defaults)"))
			} else {
				fmt.Println(ui.Render(theme.Dimmed, "# from "+p))
			}
			fmt.Print(string(data))
			return nil
		},
	}
}

func configEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config.yaml in $EDITOR (creates defaults if missing)",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			p, _, err := config.Init()
			if err != nil {
				return fmt.Errorf("ensure config exists: %w", err)
			}
			editor := pickEditor()
			if editor == "" {
				return fmt.Errorf("no editor found: set $EDITOR or $VISUAL")
			}
			ed := exec.CommandContext(c.Context(), editor, p)
			ed.Stdin = os.Stdin
			ed.Stdout = os.Stdout
			ed.Stderr = os.Stderr
			return ed.Run()
		},
	}
}

func configInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write a default config.yaml if one doesn't exist",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			p, created, err := config.Init()
			if err != nil {
				return err
			}
			if created {
				fmt.Println(ui.Render(theme.OK, "wrote "+p))
			} else {
				fmt.Println(ui.Render(theme.Dimmed, "already exists: "+p))
			}
			return nil
		},
	}
}

// pickEditor returns the user's preferred editor, falling back to a
// platform-appropriate default. We prefer $VISUAL > $EDITOR over
// hardcoded fallbacks because VISUAL is the long-standing convention
// for "interactive editor".
func pickEditor() string {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	if runtime.GOOS == "windows" {
		return "notepad"
	}
	for _, fallback := range []string{"nano", "vim", "vi"} {
		if _, err := exec.LookPath(fallback); err == nil {
			return fallback
		}
	}
	return ""
}
