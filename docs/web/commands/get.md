# sm get

Fetch a colleague's stack locally and reproduce its parent edges so `sm log` mirrors the author's tree.

## Synopsis

```
sm get <PR-number>
```

## Examples

Pull PR #142 and every branch below it:

```bash
sm get 142
```

After it returns, HEAD is on the top branch and `sm log` shows the full tree.

## What it does

1. Calls `gh pr view <num> --json baseRefName,headRefName` to find the PR's base and head branches.
2. Walks the `base_ref` chain down to trunk, calling `gh pr checkout` on each branch.
3. Records parent metadata (`branch.<n>.stac-man-parent` and `-parent-sha`) for every branch.
4. Records the PR number on each branch (`branch.<n>.stac-man-pr`).
5. Leaves HEAD on the top branch (the one PR `<num>` was opened against).
6. Prints `sm log` so you can confirm the shape.

## When to use it

- Reviewing a stacked PR another contributor opened — pull the whole stack so you can read it bottom-up.
- Resuming work on a different machine — `sm` metadata is intentionally not pushed; `sm get` is the recovery path.
- Debugging "why is PR #X targeting Y?" — the local checkout makes the relationships obvious.

## What it does NOT do

- Get does not run `sm restack`. The branches are checked out exactly as they exist on origin.
- Get does not adopt branches that aren't part of the PR base chain. If the author has unsubmitted children locally, only the submitted ones are pulled.

## See also

- [`sm submit`](./submit) — the inverse: push your local stack as PRs.
- [`sm log`](./log) — confirm the shape after `sm get`.
- [Recipes → Review someone's stack](/recipes/review-someones-stack).
