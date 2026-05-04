# sm submit

Push the current branch (or the whole stack) and open or update PRs through `gh`. Existing PRs are retargeted when their base branch has changed locally. Every PR body is auto-decorated with a sentinel-fenced **stack table** so reviewers see the full chain at the top of the description.

## Synopsis

```
sm submit [--stack] [--draft] [--body <markdown>] [--no-restack] [--no-stack-table]
```

## Flags

| Flag | Description |
|---|---|
| `--stack` | Submit every ancestor, the current branch, and every descendant. Idempotent — branches already in sync with origin are not re-pushed. |
| `--draft` | Create new PRs as drafts. Does not change the draft state of existing PRs. |
| `--body <markdown>` | PR body for newly-created PRs (existing PRs unchanged). The auto-generated stack table is then injected at the top. |
| `--no-restack` | Push branches even when their recorded parent SHA is stale. Default is to skip those branches with a "needs restack" message. With this flag they're pushed anyway and reported under `pushed with stale parent SHA (--no-restack):` so the staleness is unmissable. |
| `--no-stack-table` | Do not inject (or refresh) the auto-generated stack table at the top of PR bodies. |

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

Force a push despite a stale parent SHA (use sparingly — the resulting diff on GitHub will include parent commits until the next clean restack):

```bash
sm submit --no-restack
```

## What it does

For each branch in scope:

1. Push to origin (with `--force-with-lease`).
2. If the recorded parent SHA is stale relative to the parent's current tip, skip with a "needs restack" message — unless `--no-restack` was passed, in which case push anyway and surface the staleness in the report.
3. If no PR is recorded yet, open one through `gh pr create` and persist the PR number under `branch.<n>.stac-man-pr`.
4. If a PR is recorded but its base on GitHub differs from the recorded local parent, update the base via `gh pr edit`.

After every PR exists (one or more of the above per branch), `sm submit` runs a final pass that rewrites each PR's description to inject — or refresh — the [stack table](#pr-body-stack-table). Skipped unless `--no-stack-table` was passed.

The combined CI / mergeability cache at `.git/stac-man/checks-cache.json` is cleared at the end of submit, so the next `sm log` fetches fresh status for every PR you just touched.

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
pushed with stale parent SHA (--no-restack):
  feat/auth-tests
  → diff on GitHub may include parent commits; run `sm restack` then `sm submit` to clean up.
```

If a stack-mate has diverged from origin (someone force-pushed over it), submit warns rather than clobbering.

## PR body stack table

Reviewers who land on a PR mid-stack want one question answered first: **where am I in the chain?** `sm submit` inserts a sentinel-fenced markdown block at the top of every PR body that lists every PR in the chain trunk → leaf, bolds the row corresponding to the PR being viewed, and tags it with `← this PR`. The block is regenerated on every submit.

Sentinels:

```
<!-- stac-man:stack-start -->
...
<!-- stac-man:stack-end -->
```

Sample rendered block, for the middle of a 3-branch stack:

```markdown
<!-- stac-man:stack-start -->
**Stack** _(managed by [stac-man](https://github.com/bluegardenproject/stac-man))_

- #41 feat/auth-models
- **#42 feat/auth-handlers ← this PR**
- #43 feat/auth-docs
<!-- stac-man:stack-end -->
```

### Idempotency

Running `sm submit` again with no real changes is a no-op: the body update is skipped when the new block is byte-identical to the existing one, so `gh pr edit` doesn't fire and reviewers don't see notification noise. When the stack actually changes (a branch is added, a PR number lands, the chain reorders), the next submit replaces the existing block in place — never duplicates, never accumulates. Manual prose above and below the sentinels survives every regeneration.

### Path view, not graph view

For a non-linear stack (branch with siblings), each PR's table shows only the linear path that PR sits on: its ancestors, itself, and its descendants. Sibling subtrees are deliberately omitted because a reviewer of `feat/auth-models` learns nothing useful from a sibling branch they don't depend on.

### Single-branch stacks get no block

A branch that sits alone on trunk — no tracked descendants, no tracked ancestors below trunk — produces no stack table. There's nothing to describe and the noise hurts non-stacked PRs. As soon as you stack a second branch on top, both PRs grow the block on the next `sm submit`.

### Branches without a PR yet

If an ancestor has been tracked but not yet submitted, its row appears in the table without a number, marked `_(no PR)_`. This is the signal to the reviewer that the chain has an unsubmitted gap; the next `sm submit --stack` from the bottom fills it in.

### Opting out

Pass `--no-stack-table` to skip the body-rewrite pass entirely. Existing blocks are NOT removed — they stay until you delete them by hand or run a future submit without the flag. If you want a permanent opt-out for one PR, edit the body on GitHub and remove both sentinel comments; submit will then prepend a fresh block (you can run `--no-stack-table` from then on to keep it out).

## See also

- [`sm get`](./get) — the inverse: pull someone's stack down locally.
- [`sm land`](./land) — merge the bottom-most PR.
- [Concepts → Stacks](/concepts/stacks) — why stack PRs in the first place.
