# stac-man

`sm` — a CLI for stacked pull requests, in the spirit of Graphite's `gt`.

Documentation is being written. See `cmd/` for the command surface and `internal/` for the architecture once filled in.

## Setup

This repository ships its git hooks in [`.githooks/`](.githooks). After cloning, enable them once with:

```bash
git config core.hooksPath .githooks
```

That's it — git will now run the in-repo hooks for every commit.

### What the hooks do

- `commit-msg`: enforces [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) on the commit header, e.g. `feat(git): add typed accessors`. Merge, revert, fixup, squash, and amend commits are skipped.

If you ever need to bypass the hook for a one-off (e.g. an emergency fix), use git's standard escape hatch:

```bash
git commit --no-verify
```
