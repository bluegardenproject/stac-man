// Package tui hosts interactive Bubble Tea models used by stac-man
// commands when stdout is a TTY. Today only the branch picker (used
// by `sm checkout` without an argument) lives here; future TUI work
// (the no-args menu, `sm log -i`, the conflict resolver) will land
// in sibling files. Models read everything they need via the
// service layer, never via git/gh directly.
package tui
