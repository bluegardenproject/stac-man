// Package git wraps the local `git` executable.
//
// It exposes typed accessors (CurrentBranch, RevParse, MergeBase,
// Rebase, …) over a small Runner interface so production code shells
// out while tests can swap in a recorded fake.
//
// The package is intentionally domain-free: nothing here knows about
// stacks, parents, or PRs. Higher layers (internal/stack,
// internal/restack) layer that meaning on top.
package git
