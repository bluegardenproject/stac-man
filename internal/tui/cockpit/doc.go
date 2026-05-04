// Package cockpit hosts the interactive Bubble Tea application that
// bare `sm` launches on a TTY. It is the v2.1 evolution of the
// stand-alone branch picker in the sibling internal/tui package: a
// multi-screen dashboard over the local stack with single-key
// actions, an in-TUI conflict resolver, a diff viewer, and a command
// palette.
//
// Callers should treat Run as the only entry point. Models, screens,
// and key bindings are all internal so the package can evolve its
// shape without breaking the cmd layer.
//
// Network-bearing operations (submit, sync, land, get) shell out via
// tea.ExecProcess to the existing CLI subcommands; everything else
// runs in-process through *service.Service. See ROADMAP.md "v2.1
// Interactive TUI" and the cockpit plan in .cursor/plans/ for the
// architectural rationale.
package cockpit
