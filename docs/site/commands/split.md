# sm split

Decompose the current branch into a chain of smaller branches. Default: one branch per commit.

## Synopsis

```
sm split [--names <a,b,c> --commits <1-2,3,4-5>]
```

## Flags

| Flag | Description |
|---|---|
| `--names <a,b,c>` | Comma-separated branch names (paired with `--commits`). |
| `--commits <ranges>` | Comma-separated 1-based commit ranges per branch, e.g. `1-2,3,4-5`. |

`--names` and `--commits` must be used together. Without them, split runs in auto mode: one branch per commit, named after the slugified commit subject.

## Examples

Auto split — one branch per commit:

```bash
sm split
```

Explicit grouping:

```bash
sm split --names api-models,api-handlers,api-tests \
         --commits "1-3,4-6,7"
```

Commits are 1-based, oldest first. Run `sm show` first to see the commit list and pick the ranges.

## What it does

1. Reads the commits unique to the current branch.
2. Replaces the branch with a chain of new branches, each one stacked on the previous, picking the chosen commits.
3. Each new branch has its parent recorded automatically.
4. Leaves HEAD on the topmost new branch.

## When to use it

- A branch grew bigger than reviewers want — break it up before submitting.
- You committed several logical changes together; split into reviewable chunks.

## What it does NOT do

- Per-hunk split is intentionally not supported. If you need to route hunks to different commits, use [`sm absorb`](./absorb) (for fixups against existing commits) or `git add -p` to stage hunks first.

## See also

- [`sm fold`](./fold) — the inverse.
- [Recipes → Split a fat branch](/recipes/split-fat-branch).
