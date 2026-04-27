package cmd

import (
	"fmt"
	"os"

	"github.com/philipptpunkt/stac-man/internal/ui"
	"github.com/philipptpunkt/stac-man/internal/ui/theme"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:     "checkout [branch]",
		Aliases: []string{"co"},
		Short:   "Switch HEAD to a tracked branch (or trunk)",
		Long: "With an argument, checkout switches HEAD to the named branch. " +
			"Without an argument it lists tracked branches grouped with the trunk; on a TTY " +
			"the user is prompted to pick one.",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: branchNameCompletion,
		RunE: func(c *cobra.Command, args []string) error {
			s := newService()
			if len(args) == 1 {
				return s.Checkout(c.Context(), args[0])
			}

			current, choices, err := s.CheckoutChoices(c.Context())
			if err != nil {
				return err
			}
			fmt.Println(ui.Render(theme.Header, "Tracked branches"))
			for i, name := range choices {
				marker := "  "
				if name == current {
					marker = ui.Render(theme.OK, "→ ")
				}
				fmt.Printf("%s%2d. %s\n", marker, i+1, name)
			}

			// Non-TTY: just list, don't prompt.
			if fi, err := os.Stdin.Stat(); err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
				return nil
			}
			fmt.Print("\nNumber to checkout (Enter to cancel): ")
			var pick int
			n, _ := fmt.Fscan(os.Stdin, &pick)
			if n != 1 || pick < 1 || pick > len(choices) {
				return nil
			}
			return s.Checkout(c.Context(), choices[pick-1])
		},
	}
	register(cmd)
}
