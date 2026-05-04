# sm sync

Pull trunk, delete merged branches, restack survivors, retarget PR bases on GitHub.

## Synopsis

```
sm sync
```

No flags.

## Examples

Daily-cleanup pass:

```bash
sm sync
```

Output (typical):

```
✓ pulled main
merged & deleted:
  feat/db-schema
retargeted PR bases on GitHub:
  #42 (feat/api-endpoints) → main
✓ stack is up to date
```

## What it does, in order

1. **Fetch** from origin.
2. **Fast-forward** local trunk to `origin/<trunk>`.
3. **Detect merged branches** — any tracked branch whose unique commits are now reachable from trunk.
4. **Re-parent children** of merged branches onto the merged branch's parent (usually trunk).
5. **Delete merged branches locally.**
6. **Restack survivors** onto their (possibly new) parents.
7. **Retarget PR bases on GitHub** for any branch whose parent changed.

See [Concepts → Sync](/concepts/sync) for the mental model.

## On conflict

If restacking the survivors hits a conflict, sync pauses with a `PausedError` exactly like `sm restack`:

```
⚠ rebase paused on feat/handlers
```

Resolve, `git add`, then [`sm continue`](./continue).

## What sync does NOT do

- Push. If sync changed your local stack, run [`sm submit`](./submit) `--stack` after.
- Touch trunk if it's diverged from origin (refuses with an error rather than risk a non-fast-forward).

## See also

- [Concepts → Sync](/concepts/sync) — the full step-by-step.
- [`sm land`](./land) — merge the bottom-most PR and run sync as cleanup.
- [`sm restack`](./restack) — the local-only counterpart.
