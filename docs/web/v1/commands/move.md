> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm move

Reparent a branch (and its subtree) onto a new base. Descendants ride along.

## Synopsis

```
sm move [branch] --onto <new-parent>
```

If no branch is given, operates on the current branch. `--onto` is required.

## Flags

| Flag | Description |
|---|---|
| `--onto <branch>` | New parent branch. Required. Tab-completes to tracked branches + trunk. |

## Examples

Move the current branch and its subtree onto trunk:

```bash
sm move --onto main
```

Move a specific branch onto a sibling:

```bash
sm move feat/web-form --onto feat/api-v2
```

## What it does

1. Reassigns `branch.<n>.stac-man-parent` to `--onto`.
2. Runs the restack engine over the named branch AND every descendant. Each branch is rebased onto its (possibly new) parent's tip.
3. On conflict, pauses with a `PausedError` — same protocol as [`sm restack`](./restack).

## move vs. parent --set

| | `sm move --onto` | [`sm parent --set`](./parent) |
|---|---|---|
| Reparents a single branch? | Yes | Yes |
| Reparents descendants too? | Yes | No |

Use `sm move` when you mean "move this subtree somewhere else"; use `sm parent --set` for fine-grained surgery.

## See also

- [`sm parent`](./parent) — single-branch reparent.
- [`sm split`](./split) — when "move" turns out to mean "this should be several branches."
- [Recipes → Recover from a bad rebase](/v1/recipes/recover-from-bad-rebase).
