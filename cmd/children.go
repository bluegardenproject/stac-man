package cmd

import (
	"fmt"

	"github.com/bluegardenproject/stac-man/internal/ui"
	"github.com/bluegardenproject/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "children [branch]",
		Short: "List the direct children of a branch",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			children, err := newService().Children(c.Context(), branch)
			if err != nil {
				return err
			}
			if len(children) == 0 {
				fmt.Println(ui.Render(theme.Dimmed, "(no children)"))
				return nil
			}
			for _, name := range children {
				fmt.Println(ui.Render(theme.Accent, name))
			}
			return nil
		},
	}
	register(cmd)
}
