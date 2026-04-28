# Troubleshooting

When something feels off, run [`sm doctor`](/commands/doctor) first. It's read-only and will tell you exactly what's drifted. The table below covers the cases it catches plus a few it doesn't.

## "needs restack" / "stale parent SHA"

A parent moved without its descendants catching up.

```bash
sm restack
```

If conflicts pop up, follow the [pause protocol](/concepts/restack#pause-and-resume).

## "drifted parent SHA"

A branch's tip rewrote outside `sm` (e.g. you ran `git rebase` directly, or someone force-pushed over it).

```bash
sm restack            # usually fixes it
sm doctor             # confirm
```

If `sm restack` can't reconcile (rare), see [Recover from a bad rebase → Symptom 2](/recipes/recover-from-bad-rebase#symptom-2-finished-but-the-chain-looks-wrong).

## "untracked branches with unique commits"

You created a branch with `git switch -c` and `sm` doesn't know about it. Either adopt or ignore:

```bash
sm checkout my-branch
sm track              # adopt with auto-detected parent
# or just ignore — `sm doctor` only reports it; nothing breaks
```

## "graph issues"

Should not happen in steady state. If you see a cycle or a missing parent, that's a bug — please file an issue with the output of:

```bash
git config --list --local | grep stac-man
sm doctor
```

## `sm` says "no controlling terminal" / picker doesn't open

`sm checkout` (no arg) opens a Bubble Tea picker on a TTY. If stdin or stdout is redirected, it falls back to printing the static tree.

```bash
sm checkout            # works in your terminal
sm checkout | grep .   # falls back to static tree (intended)
```

Force the static tree explicitly:

```bash
sm log
```

## `sm submit` says "stack-mate diverged from origin"

Someone (or you, on another machine) pushed over a branch in your stack. `sm submit` refuses to clobber. Recover by:

```bash
git fetch
sm get <PR-of-the-diverged-branch>     # pull their version
# or, if you're sure your local is right:
git push --force-with-lease origin <branch>
sm submit --stack
```

## A paused restack won't resume

If `.git/stac-man/restack.json` is malformed or stale (rare), `sm abort` always works:

```bash
sm abort
```

This runs `git rebase --abort` if needed and clears the queue.

## `gh` not authenticated

Commands that talk to GitHub (`submit`, `land`, `get`, the PR-status part of `log`) need `gh` authenticated:

```bash
gh auth status
gh auth login        # if not already
```

`sm` itself never stores GitHub tokens — that's `gh`'s job.

## `sm log` is slow

The `gh pr view` round-trip per branch with a recorded PR adds up. Skip it:

```bash
sm log --no-pr
```

The local tree renders instantly.

## "git-absorb: command not found"

`sm absorb` requires the upstream `git-absorb` binary. Install it:

```bash
brew install git-absorb              # macOS
cargo install git-absorb              # other platforms
```

Then re-run `sm absorb`.

## I deleted a branch with `git branch -D` and now `sm` is confused

`sm`'s metadata still references it. Either re-create the branch (and re-track), or drop the metadata:

```bash
sm untrack <name> --reparent      # if it had children you want to keep
# or, if you don't care about children:
git config --local --remove-section "branch.<name>"
```

`sm doctor` will confirm it's clean.

## Where things live, when in doubt

- Stack metadata: `git config --list --local | grep stac-man`
- Trunk: `git config --local --get stac-man.trunk`
- Paused-restack queue: `.git/stac-man/restack.json`
- Undo log: `.git/stac-man/history.json`
- User config (optional): `~/.config/stac-man/config.yaml`

## Still stuck

Open an issue on [github.com/philipptpunkt/stac-man](https://github.com/philipptpunkt/stac-man/issues) with the output of:

```bash
sm version
sm doctor
git config --list --local | grep stac-man
```
