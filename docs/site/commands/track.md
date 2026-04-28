# sm track

Adopt an existing branch into the stack graph.

## Synopsis

```
sm track [branch] [--parent <branch>]
```

If no branch is given, tracks the current branch.

## Flags

| Flag | Description |
|---|---|
| `--parent <branch>` | Explicit parent branch. Default: inferred from `git merge-base`. |

## Examples

You created a branch with plain `git switch -c` and want `sm` to know about it:

```bash
git switch -c feat/legacy-import
sm track --parent main
```

Track the current branch with auto-detected parent:

```bash
sm track
```

Track a different branch by name:

```bash
sm track feat/legacy-import --parent feat/api-v2
```

## What it does

1. Resolves the parent (explicit `--parent` wins; otherwise `git merge-base` against tracked branches and trunk picks the closest match).
2. Records `branch.<n>.stac-man-parent` and `branch.<n>.stac-man-parent-sha`.
3. Prints the resolved parent.

After tracking, the branch shows up in `sm log`, gets restacked by `sm restack` / `sm sync`, and is eligible for `sm submit`.

## See also

- [`sm untrack`](./untrack) — the inverse.
- [`sm create`](./create) — preferable to `git switch -c` + `sm track` when you're starting fresh.
- [`sm doctor`](./doctor) — surfaces untracked branches with unique commits.
