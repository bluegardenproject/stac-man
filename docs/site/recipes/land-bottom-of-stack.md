# Land the bottom of a stack

The bottom-most PR has approvals and green CI. You want to land it AND keep working on the rest of the stack without manual cleanup.

## Starting state

```
main
└── feat/auth-models       (PR #41, OPEN, approved, green)
    └── feat/auth-handlers (PR #42, OPEN)
        └── feat/auth-docs (PR #43, DRAFT)
```

You're sitting on `feat/auth-docs`.

## Step 1 — confirm the bottom is ready

```bash
sm log
```

If a PR is `DRAFT` or has red CI, decide whether you really want to land it before the next steps. `sm land` will refuse a red PR unless you pass `--force`.

## Step 2 — land

```bash
sm land
```

What happens, in order:

1. `sm` walks toward trunk and finds the bottom-most tracked PR (`#41`).
2. `gh pr checks` confirms CI is green.
3. `gh pr merge --squash` merges the PR on GitHub.
4. `sm sync` runs:
   - Fetches origin.
   - Fast-forwards `main`.
   - Detects `feat/auth-models` as merged and deletes it locally.
   - Re-parents `feat/auth-handlers` onto `main`.
   - Restacks `feat/auth-handlers` and `feat/auth-docs` onto the new `main`.
   - Calls `gh pr edit` on `#42` to retarget its base from `feat/auth-models` to `main`.

## Ending state

```
main
└── feat/auth-handlers (PR #42, base = main)
    └── feat/auth-docs (PR #43)
```

`feat/auth-models` is gone locally. PR #41 is merged. PR #42's base now points at `main` so reviewers see only the handler diff.

## Variations

### Pick a strategy

```bash
sm land --merge     # merge commit
sm land --rebase    # rebase merge
```

Default is `--squash`. The flags are mutually exclusive.

### Skip the CI gate

```bash
sm land --force
```

For when CI is broken on `main`, not on the PR — or any other case where you've decided red CI is acceptable.

### From a branch other than the top

You don't need to be on the top branch. As long as you're somewhere in the stack:

```bash
sm checkout feat/auth-handlers
sm land           # still finds #41 as the bottom
```

## What if restack pauses

If retargeting causes a conflict (rare on a healthy stack):

```
⚠ rebase paused on feat/auth-handlers
```

Resolve, `git add`, then [`sm continue`](/commands/continue). The PR has already merged on GitHub at this point — only the local cleanup is paused.

## See also

- [`sm land`](/commands/land) — flag reference.
- [`sm sync`](/commands/sync) — what's running under the hood.
- [Recipes → Recover from a bad rebase](./recover-from-bad-rebase) — escape hatches if anything goes sideways.
