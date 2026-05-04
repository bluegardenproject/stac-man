# sm doctor

Sanity-check stac-man metadata vs. git state. Read-only — never mutates anything.

Alias: `sm status`.

## Synopsis

```
sm doctor
```

No flags.

## Example output (healthy)

```
stac-man doctor

  → main (trunk)
  → 4 tracked branches

  ✓ everything looks healthy
```

## Example output (drifted)

```
stac-man doctor

  → main (trunk)
  → 4 tracked branches

needs restack:
  feat/auth-handlers
  → run `sm restack`

stale parent SHAs:
  feat/auth-tests
  → run `sm restack` to refresh

untracked branches with unique commits:
  experimental/sketch
  → checkout each and run `sm track` if you want it in the stack
```

## What it checks

| Issue | Meaning | Suggested fix |
|---|---|---|
| `PRs with merge conflicts` | GitHub reports the PR as `CONFLICTING` — typically when an ancestor PR's base on origin is stale after a sync. | Resolve on GitHub, or `sm sync && sm restack && sm submit`. |
| `needs restack` | Parent SHA differs from parent's current tip. | `sm restack` |
| `stale parent SHA` | Recorded SHA is no longer reachable. | `sm restack` |
| `drifted parent SHA` | Branch tip rewrote outside `sm`. | `sm restack` or fix manually |
| `untracked branches with unique commits` | Branch you might want in the stack but haven't tracked. | `sm checkout <name> && sm track` |
| `graph issues` | Cycles, missing parents, etc. — should not happen in steady state. | File a bug |

The `PRs with merge conflicts` check issues a best-effort `gh pr view` per tracked branch with a recorded PR number, sharing the 60s cache `sm log` already populates at `.git/stac-man/checks-cache.json`. Drafts and merged or closed PRs are skipped — only PRs the user is actively preparing to land are surfaced. If `gh` is unavailable or unauthenticated, doctor silently omits the block instead of failing; doctor must always work offline.

## When to run it

- Whenever something looks off in `sm log`.
- Before opening a stack of PRs (catches misconfigured parents early).
- After force-pushes or wholesale `git rebase` work outside `sm`.

## See also

- [Concepts → Parent metadata](/concepts/parent-metadata) — what's being checked.
- [`sm restack`](./restack) — the most common fix.
- [`sm undo`](./undo) — when the fix is "rewind."
