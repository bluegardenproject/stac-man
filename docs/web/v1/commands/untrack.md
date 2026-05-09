> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm untrack

Remove a branch from the stack graph. Does not delete the branch itself.

## Synopsis

```
sm untrack [branch] [--reparent]
```

If no branch is given, untracks the current branch.

## Flags

| Flag | Description |
|---|---|
| `--reparent` | If the branch has tracked children, move them onto its parent. |

## Examples

Drop tracking for the current branch:

```bash
sm untrack
```

Untrack a branch but keep its children in the stack:

```bash
sm untrack feat/scratch --reparent
```

Without `--reparent`, untracking a branch that has tracked children is refused — the children would be orphaned.

## What it does

1. Removes `branch.<n>.stac-man-parent`, `-parent-sha`, and `-pr` from `.git/config`.
2. With `--reparent`: re-points each tracked child's parent to the untracked branch's parent and restacks them.
3. Leaves the branch itself untouched in git.

## When to use it

- You created a branch you don't actually want to ship as a PR.
- A branch was tracked by mistake.
- You're cleaning up after experimenting with a stack shape that didn't pan out.

## See also

- [`sm track`](./track) — the inverse.
- [`sm fold`](./fold) — collapse rather than untrack when the work itself should land.
