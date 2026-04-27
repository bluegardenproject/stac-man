// Package stack is the pure domain layer: branches, the parent graph,
// and the operations on it (children, ancestors, descendants, topo
// sort, validate). It depends only on internal/store, not on git or
// gh, so it can be unit-tested with an in-memory store.
package stack
