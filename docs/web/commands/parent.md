# sm parent

Show or change the parent of a branch.

## Synopsis

```
sm parent [branch] [--set <new-parent>]
```

If no branch is given, operates on the current branch.

## Flags

| Flag | Description |
|---|---|
| `--set <branch>` | Reassign parent to the given branch and restack. |

## Examples

Print the current branch's parent:

```bash
sm parent
# feat/auth-models
```

Print another branch's parent:

```bash
sm parent feat/auth-handlers
# feat/auth-models
```

Reassign and restack:

```bash
sm parent --set feat/db-schema
```

## What `--set` does

1. Updates `branch.<n>.stac-man-parent` to the new value.
2. Runs the restack engine to rebase the branch (and only this branch) onto the new parent's tip.
3. On conflict, pauses with a `PausedError` — same protocol as [`sm restack`](./restack).

## parent vs. move

| | `sm parent --set` | [`sm move --onto`](./move) |
|---|---|---|
| Reparents a single branch? | Yes | Yes |
| Reparents the subtree (descendants too)? | No — only the named branch is restacked | Yes — descendants ride along |

Use `sm parent --set` when only one branch is moving; use `sm move` when the whole subtree should follow.

## See also

- [`sm move`](./move) — subtree reparent.
- [`sm children`](./children) — list children of a branch.
