# Parent metadata

`sm` records every stack relationship in `.git/config`. There is no separate database, no `.stacman/` directory, no remote service. If you want to know what `sm` knows, run `git config --list --local | grep stac-man`.

## The keys

For every tracked branch:

```ini
[branch "feat/api-endpoints"]
    stac-man-parent = feat/db-schema       # who am I stacked on
    stac-man-parent-sha = 4f1e2a91…        # tip of parent at last restack
    stac-man-pr = 42                       # GitHub PR number, if submitted
```

And one repo-level key:

```ini
[stac-man]
    trunk = main                           # see Concepts → Trunk
    version = 2                            # metadata schema version
```

That's the source-of-truth metadata surface in steady state. Derived GitHub data may also be cached under `.git/stac-man/cache.db`, but the stack graph itself remains in git config.

## What each key means

### `branch.<n>.stac-man-parent`

The name of the parent branch. May be trunk or another tracked branch. If absent, the branch is **untracked** — `sm` ignores it.

### `branch.<n>.stac-man-parent-sha`

The tip of the parent branch the last time this branch was rebased onto it. The restack engine compares the parent's current tip to this value to decide if work is needed.

If the SHAs match: this branch is already on top of its parent's current tip. Skip.

If they differ: the parent moved (someone amended, the user ran `sm modify`, sync fast-forwarded trunk, …). Rebase, then update this key.

### `branch.<n>.stac-man-pr`

The GitHub PR number, recorded when `sm submit` opens or adopts a PR. Used by `sm log`, `sm show`, `sm land`, and `sm sync --retarget-bases`. Can be re-bound by re-running `sm submit`.

## The transient files

Additional local state lives under `.git/stac-man/`:

- `restack.json` — the in-flight queue while a restack is paused on a conflict. Cleared by `sm continue` (when the queue drains) or `sm abort`.
- `history.json` — the rolling undo log, capped at 50 entries. `sm undo` consumes it.
- `cache.db` — a disposable SQLite cache for GitHub-derived PR/status data.

You should not edit these by hand. If `restack.json` is malformed, `sm abort` is always safe.

## What `sm doctor` checks

`sm doctor` re-derives the truth from git itself and compares it to the metadata above. It reports:

| Issue | What it means | Fix |
|---|---|---|
| `needs restack` | Parent SHA differs from parent's current tip. | `sm restack` |
| `stale parent SHA` | Recorded SHA is no longer reachable. | `sm restack` |
| `drifted parent SHA` | Branch's tip rewrote outside `sm`. | `sm restack`, or fix manually |
| `untracked branch with unique commits` | A branch you might want in the stack but haven't tracked. | `sm checkout <name> && sm track` |
| `graph issues` | Cycles, missing parents, etc. — should be impossible in steady state. | Surface as a bug |

Run `sm doctor` whenever the stack feels off. It's read-only.

## Why git config

- It's already there. No new file format to learn, no new place state can drift to.
- `git config` is the same on every platform.
- Cloning the repo elsewhere wipes `sm` state automatically — the metadata is intentionally per-clone, not pushed to origin.
- `sm get <PR-number>` is the recovery path for moving a stack between machines: it walks the GitHub PR's `base_ref` chain and re-records the keys on the new clone.

## Why not a remote source of truth

`sm` deliberately never pushes its own metadata. Two reasons:

1. **No SaaS, no login.** That's the project's whole point.
2. **The PR base graph on GitHub already holds the same shape.** If you lose `.git/config`, `sm get <PR>` reconstructs it.

If you want a richer collaborative experience, use a hosted product. `sm` is for the local-first half of the spectrum.
