> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm absorb

Auto-route uncommitted hunks into the right ancestor commits via [`git-absorb`](https://github.com/tummychow/git-absorb), then restack descendants.

## Synopsis

```
sm absorb [--base <branch>]
```

## Flags

| Flag | Description |
|---|---|
| `--base <branch>` | Use this branch as the absorb base. Default: the lowest tracked ancestor. |

## Prerequisites

`git-absorb` must be on `PATH`. Install it via your package manager — `brew install git-absorb`, `cargo install git-absorb`, etc. If it's missing, `sm absorb` prints a friendly install hint and exits.

## Examples

You've made small fixes to several places in the stack and they belong in earlier commits. Stage nothing, then:

```bash
sm absorb
```

`git-absorb` matches each hunk to the right historical commit (the one that introduced the surrounding code) and creates `fixup!` commits, then runs `git rebase --autosquash` to fold them in. `sm` then restacks descendants.

Pick a specific base:

```bash
sm absorb --base feat/api-models
```

## What it does

1. Calls `git absorb --base <branch>` on the working tree's uncommitted changes.
2. Restacks every descendant of the affected branch so they pick up the new commits.
3. On conflict during the descendant rebase, pauses with a `PausedError`.

## When this beats manual fixups

Without `sm absorb`, the equivalent is the three-step template below — one `git commit --fixup` per hunk, an interactive autosquash rebase, and a stac-man restack:

```text
git commit --fixup=<sha-of-target-commit>   # repeat per hunk
git rebase -i --autosquash <base-branch>
sm restack
```

`sm absorb` does all three steps with one command.

## See also

- [Recipes → Absorb fixups](/v1/recipes/absorb-fixups).
- [`sm modify`](./modify) — when changes belong on the current tip, not in an ancestor.
