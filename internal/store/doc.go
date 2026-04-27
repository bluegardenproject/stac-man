// Package store defines the metadata Store interface used by
// internal/stack. Two implementations live underneath:
//
//   - store/gitconfig: production, persists to git config keys.
//   - store/memory:    test fake.
//
// Keeping the interface narrow (Get/Set/Unset/List per branch + repo)
// means the stack layer never has to know whether it's talking to git
// or to a map.
package store
