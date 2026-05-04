# sm fold

Squash the current branch into its parent. Useful once a stack entry's history has settled and you want a clean linear parent.

## Synopsis

```
sm fold [-m <msg>]
```

## Flags

| Flag | Description |
|---|---|
| `-m`, `--message <msg>` | Commit message for the squashed commit. Default: `"fold <branch> into <parent>"`. |

## Examples

```bash
sm fold -m "auth: models + handlers"
```

## What it does

1. Squashes every commit unique to the current branch into a single commit on its parent.
2. Deletes the (now-folded) branch locally.
3. Re-parents any tracked children onto the parent.
4. Restacks the children so they pick up the new tip.

Effectively the inverse of [`sm split`](./split): split decomposes a branch into a chain of smaller ones; fold collapses a branch into its parent.

## When NOT to use it

- When the branch has an open PR you still want to land separately. Folding deletes the branch — `gh` won't be happy.
- When you mean "merge this PR." Use [`sm land`](./land) instead.

## See also

- [`sm split`](./split) — the inverse.
- [`sm land`](./land) — merge through GitHub instead of locally folding.
