# sm submit

Push the current branch (or the whole stack) and open or update PRs through `gh`. Existing PRs are retargeted when their base branch has changed locally.

## Synopsis

```
sm submit [--stack] [--draft] [--body <markdown>]
```

## Flags

| Flag | Description |
|---|---|
| `--stack` | Submit every ancestor, the current branch, and every descendant. Idempotent — branches already in sync with origin are not re-pushed. |
| `--draft` | Create new PRs as drafts. Does not change the draft state of existing PRs. |
| `--body <markdown>` | PR body for newly-created PRs (existing PRs unchanged). |

## Examples

Submit the current branch only:

```bash
sm submit
```

Submit the entire stack you're sitting on:

```bash
sm submit --stack
```

Open new PRs as drafts:

```bash
sm submit --stack --draft
```

## What it does

For each branch in scope:

1. Push to origin (with `--force-with-lease`).
2. If no PR is recorded yet, open one through `gh pr create` and record `branch.<n>.stac-man-pr`.
3. If a PR is recorded but its base on GitHub differs from the recorded local parent, update the base via `gh pr edit`.

Pre-submit, `--stack` runs an implicit [`sm restack`](./restack) so origin matches the local chain.

## Output

```
pushed:
  feat/auth-models
  feat/auth-handlers
created PRs:
  #41 feat/auth-models
updated PRs:
  #42 feat/auth-handlers
already in sync with origin:
  feat/web-form
```

If a stack-mate has diverged from origin (someone force-pushed over it), submit warns rather than clobbering it.

## See also

- [`sm get`](./get) — the inverse: pull someone's stack down locally.
- [`sm land`](./land) — merge the bottom-most PR.
- [Concepts → Stacks](/concepts/stacks) — why stack PRs in the first place.
