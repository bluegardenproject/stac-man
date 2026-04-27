// Package restack orchestrates the rebase walk for `sm restack` and
// `sm sync`. On conflict it persists resume state to
// .git/stac-man/restack.json so `sm continue` / `sm abort` can pick
// the operation back up where it stopped.
package restack
