# Recover from a bad rebase

A rebase paused on conflicts and you're not sure what to do, or you finished a restack and now `sm log` looks wrong. This page is the escape-hatch reference.

## Symptom 1 — paused on conflict

You see:

```
⚠ rebase paused on feat/handlers
  resolve the conflicts, `git add` the resolved files, then run:
    sm continue
  or to bail out:
    sm abort
```

### To resolve and continue

```bash
$EDITOR <conflicted files>
git add <resolved files>
sm continue
```

`sm continue` runs `git rebase --continue` AND processes the rest of the queue. If another conflict shows up later in the chain, repeat.

### To bail out

```bash
sm abort
```

Discards the in-flight rebase AND the queue. Restores whatever branch you were on before the operation started.

::: warning
Don't run plain `git rebase --continue` here. The restack engine has more work queued; only `sm continue` knows what's left.
:::

## Symptom 2 — finished, but the chain looks wrong

You finished a `sm modify` / `sm sync` / etc. and `sm log` shows a branch you didn't expect to move, or two branches now share commits they shouldn't.

### Step 1 — diagnose

```bash
sm doctor
```

This is read-only and will tell you exactly what's drifted: needs-restack branches, stale parent SHAs, drifted parent SHAs, untracked branches with unique commits.

### Step 2 — try the obvious fix

Most issues `sm doctor` reports are fixed by:

```bash
sm restack
```

If that completes cleanly, run `sm doctor` again. Healthy? You're done.

### Step 3 — undo the last operation

If `sm restack` doesn't clean things up — or if the problem is "I just ran a bad command" — rewind it:

```bash
sm undo --dry-run     # preview
sm undo               # actually rewind
```

`sm undo` restores tip SHAs, parent metadata, and tracking state for every branch the last op touched. It uses git's reflog under the hood, so even branches that were deleted as part of the original op can usually come back.

`sm undo` keeps the last 50 operations. You can call it multiple times to peel back further:

```bash
sm undo
sm undo
sm undo
```

### Step 4 — last resort: manual git

If `sm undo` can't get you back (e.g. the bad op happened more than 50 ops ago), git's reflog still has the SHAs:

```bash
git reflog show feat/handlers
# 8a4f3c1 HEAD@{0}: rebase: handlers: implement /login
# 3e2c1f4 HEAD@{1}: commit: handlers: implement /login
# ...
git switch feat/handlers
git reset --hard 3e2c1f4    # restore tip
sm restack                  # let `sm` clean up the metadata
```

After any manual `git reset` of a tracked branch, run [`sm doctor`](/commands/doctor) and then [`sm restack`](/commands/restack) so `sm`'s view catches up.

## Symptom 3 — `restack.json` looks weird

The file `.git/stac-man/restack.json` shouldn't exist outside of an actively-paused operation. If it does (e.g. you `Ctrl+C`'d out of `sm continue`):

```bash
sm abort        # always safe — clears restack.json AND aborts any git rebase
```

You should never edit `restack.json` by hand.

## What to never do

- **Don't run `git rebase --continue` while `sm` is paused.** The restack engine has more work queued.
- **Don't `git branch -D` a tracked branch without `sm untrack` first.** The metadata will dangle. (Recoverable via `sm doctor` after the fact, but avoidable.)
- **Don't `git push --force`.** Use [`sm submit`](/commands/submit) so the force becomes `--force-with-lease`.

## See also

- [`sm continue` / `sm abort`](/commands/continue).
- [`sm undo`](/commands/undo).
- [`sm doctor`](/commands/doctor).
- [Concepts → Restack](/concepts/restack#pause-and-resume).
