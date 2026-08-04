# Sync

**Sync** is the daily-cleanup verb. One command does everything you'd otherwise do across `git fetch`, `git pull`, `git branch -d`, and a manual restack.

## What `sm sync` does, in order

1. **Fetch** from origin.
2. **Fast-forward** the local trunk to `origin/<trunk>`. (Refuses if your trunk has diverged.)
3. **Detect merged branches.** Any tracked branch whose unique commits are now reachable from trunk is considered merged.
4. **Re-parent children of merged branches** onto the merged branch's parent (which is usually trunk now). This is what keeps the rest of your stack hanging together when the bottom merges.
5. **Delete merged branches locally.**
6. **Restack survivors.** Every remaining tracked branch is rebased onto its (possibly new) parent's tip.
7. **Retarget PR bases on GitHub** for any branch whose parent changed during step 4 (so reviewers see the new base).
8. **Refresh cached GitHub metadata** for tracked PRs so local-first commands and the cockpit can reuse the latest known PR/status rows.

## Mental model

```
Before sync                     After sync (PR #41 merged)

main ── X ── X (origin)         main ──────────── X' (origin, fast-forwarded)
       \                                \
        Y (feat/models PR #41)           Z' (feat/handlers, reparented onto main)
         \
          Z (feat/handlers PR #42)
```

`feat/models` is gone locally. `feat/handlers` now sits directly on trunk, both locally and on GitHub. PR #42's base on GitHub is now `main`.

## When it pauses

If restacking the survivors hits a conflict, `sm sync` returns a `PausedError` exactly like `sm restack`:

```
⚠ rebase paused on feat/handlers
```

Resolve, `git add`, then `sm continue` — same protocol.

## What `sm sync` is NOT

- It is not `git pull --rebase`. `git pull --rebase` only rebases the current branch; sync rebases the whole chain.
- It is not destructive on its own — every branch deletion is gated by "this branch's commits are fully in trunk." If a branch has unique commits left, it survives.
- It does not push. If sync changed your local stack, run `sm submit --stack` after.

## Sync vs. restack

| | `sm sync` | `sm restack` |
|---|---|---|
| Touches origin? | Yes (fetch + retarget PRs) | No |
| Updates trunk? | Yes (fast-forward) | No |
| Deletes merged branches? | Yes | No |
| Rebases descendants? | Yes | Yes |
| Refreshes GitHub status cache? | Yes | No |
| Cost | Higher (network) | Local only |

Use `sm sync` once a day or when something landed on origin. Use `sm restack` for purely-local churn.

## Sync vs. status

Use `sm sync` when repository state may have changed: trunk advanced, PRs merged, or bases need retargeting. Use [`sm status`](/commands/status) when you only need GitHub's current checks and mergeability for the stack you already have locally.

## Configuring

There's no flag to disable any of these steps right now. If you want a "fetch only" pass, use `git fetch` directly; if you want a "restack only" pass, use `sm restack`.
