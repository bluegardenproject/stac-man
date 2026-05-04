# Restack

**Restacking** is rebasing a chain of branches onto their parents' current tips. It's the operation that keeps a stack consistent after any history-changing edit.

```
Before                          After

A -- B (feat/models)             A -- B' (feat/models)
       \                                \
        C -- D (feat/handlers)           C' -- D' (feat/handlers)
```

If you amend `B` into `B'`, the descendants `C` and `D` are now orphaned. Restacking rewrites them into `C'` and `D'` so the chain is contiguous again.

## When it runs

Most of the time you don't call `sm restack` directly. Restack runs implicitly inside the commands that change history:

- `sm modify` — after every commit / amend.
- `sm sync` — after pruning merged branches.
- `sm absorb` — after `git-absorb` rewrites ancestors.
- `sm move` — after a parent reassignment.
- `sm parent --set` — after reassigning a single parent.
- `sm submit --stack` — implicitly before pushing, so origin matches the local chain.

`sm restack` is the explicit form. Use it when you want to refresh the chain without changing anything else.

## How the engine walks

The chain is processed in topological order from the root down:

1. For each branch (parent first, then each child), check if its `parent-sha` matches the parent's current tip.
2. If yes, skip. The branch is up to date.
3. If no, rebase the branch onto the parent's current tip, then update `parent-sha`.

Only branches that need work get touched. Restacking is idempotent.

## Pause and resume

If a rebase hits a conflict, the engine pauses with a `PausedError`:

```
⚠ rebase paused on feat/handlers
  resolve the conflicts, `git add` the resolved files, then run:
    sm continue
  or to bail out:
    sm abort
```

What you do next:

1. Open the conflicted files in your editor and fix them.
2. Stage the resolved files with `git add <files-you-fixed>` (or `git add -A`).
3. Pick up where the engine left off:

```bash
sm continue
```

`sm continue` runs `git rebase --continue` AND processes the rest of the queue. To bail out instead, run `sm abort` — it runs `git rebase --abort` AND clears the queue, restoring whatever branch you were on.

::: warning
Don't run plain `git rebase --continue` here. The engine has more work queued after the current branch finishes; only `sm continue` knows about it.
:::

## Resume state on disk

While paused, `.git/stac-man/restack.json` records the queue, the original branch, and what's left to do. If you `Ctrl+C` out of `sm continue`, the file persists, and the next `sm continue` picks up correctly.

You should never edit `restack.json` by hand. If it gets corrupted (rare), `sm abort` is always safe.

## What restack will NOT do

- Restack will not push. Use `sm submit --stack` after the chain is consistent if you want the rewrites to land on origin.
- Restack will not rewrite trunk. Trunk is the floor; the engine stops there.
- Restack will not reorder commits within a branch. That's `sm split` (decompose) or `sm fold` (collapse).
