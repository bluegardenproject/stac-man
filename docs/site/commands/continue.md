# sm continue / sm abort

Two complementary commands for resolving a paused restack. Whenever `sm restack`, `sm modify`, `sm sync`, `sm absorb`, or `sm submit` hit a conflict, they pause with a `PausedError` and hand control back to you.

## Synopses

```
sm continue
sm abort
```

Neither takes flags or arguments.

## sm continue

Resume the paused walk after you've resolved conflicts and staged the result.

```bash
$EDITOR <conflicted files>
git add <resolved files>
sm continue
```

This runs `git rebase --continue` AND processes the rest of the queued branches. If another conflict comes up later in the chain, `sm continue` pauses again — repeat until the queue drains.

::: warning
Don't run plain `git rebase --continue` here. The engine has more work queued; only `sm continue` knows what's next.
:::

## sm abort

Bail out of a paused restack. Runs `git rebase --abort` AND clears the queue, restoring the original branch you were on when the operation started.

```bash
sm abort
```

After abort, the chain is whatever it was before the paused operation began. Nothing is half-done.

## Where the paused state lives

`.git/stac-man/restack.json` — a small JSON file with the queue, the original branch, and what's left to do. You should not edit it. If it ever gets corrupted, `sm abort` is always safe and clears it.

## See also

- [Concepts → Restack](/concepts/restack#pause-and-resume) — the resume contract in depth.
- [`sm restack`](./restack) — kick off an explicit restack.
- [`sm doctor`](./doctor) — verify the chain is clean afterwards.
