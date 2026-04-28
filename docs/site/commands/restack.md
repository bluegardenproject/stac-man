# sm restack

Rebase the current branch (or a named branch) and every descendant onto their parents' current tips. Pauses on conflict.

Alias: `sm rs`.

## Synopsis

```
sm restack [branch]
```

If no branch is given, the chain rooted at the current branch is restacked.

## Examples

After amending a commit somewhere in the middle of a stack:

```bash
sm restack
```

Restack starting from a specific branch:

```bash
sm restack feat/auth-models
```

## What it does

Walks the chain rooted at the given branch in topological order, rebasing each branch onto its parent's tip. Branches whose `parent-sha` already matches their parent's current tip are skipped — restack is idempotent.

## On conflict

The walk pauses with a `PausedError`. The CLI prints:

```
⚠ rebase paused on feat/handlers
  resolve the conflicts, `git add` the resolved files, then run:
    sm continue
  or to bail out:
    sm abort
```

See [`sm continue` / `sm abort`](./continue).

## When you don't need this

Most history changes go through commands that already restack on your behalf:

| Command | Restacks descendants? |
|---|---|
| `sm modify` | Yes |
| `sm absorb` | Yes |
| `sm move` | Yes |
| `sm parent --set` | Yes |
| `sm sync` | Yes |
| `sm submit --stack` | Yes (implicitly, before push) |

You explicitly call `sm restack` when you've changed history with plain `git` (e.g. `git rebase -i`, `git commit --amend`) and want the chain to catch up.

## See also

- [Concepts → Restack](/concepts/restack) — the engine in depth.
- [`sm continue`](./continue) — resume after resolving conflicts.
- [`sm doctor`](./doctor) — diagnose what's drifted.
