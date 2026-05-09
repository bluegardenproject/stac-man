> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# Troubleshooting

When something feels off, run [`sm doctor`](/v1/commands/doctor) first. It's read-only and will tell you exactly what's drifted. The table below covers the cases it catches plus a few it doesn't.

## "needs restack" / "stale parent SHA"

A parent moved without its descendants catching up.

```bash
sm restack
```

If conflicts pop up, follow the [pause protocol](/v1/concepts/restack#pause-and-resume).

## "drifted parent SHA"

A branch's tip rewrote outside `sm` (e.g. you ran `git rebase` directly, or someone force-pushed over it).

```bash
sm restack            # usually fixes it
sm doctor             # confirm
```

If `sm restack` can't reconcile (rare), see [Recover from a bad rebase → Symptom 2](/v1/recipes/recover-from-bad-rebase#symptom-2-finished-but-the-chain-looks-wrong).

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

In an interactive terminal:

```bash
sm checkout
```

Piped (or any non-TTY context) it falls back to the static tree:

```bash
sm checkout | grep .
```

Force the static tree explicitly:

```bash
sm log
```

## `sm submit` says "stack-mate diverged from origin"

Someone (or you, on another machine) pushed over a branch in your stack. `sm submit` refuses to clobber. Recover with one of two paths.

First, refresh your view of origin:

```bash
git fetch
```

Then either pull their version of the branch (replace `<PR-of-the-diverged-branch>` with the PR number):

```text
sm get <PR-of-the-diverged-branch>
```

…or, if you're sure your local is right, force the local chain over origin and resubmit (replace `<branch>` with the branch name):

```text
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

`sm absorb` requires the upstream `git-absorb` binary. Install it from one of the sources below.

On macOS via Homebrew:

```bash
brew install git-absorb
```

On any platform with `rustup` / `cargo`:

```bash
cargo install git-absorb
```

Then re-run `sm absorb`.

## I deleted a branch with `git branch -D` and now `sm` is confused

`sm`'s metadata still references it. Either re-create the branch (and re-track), or drop the metadata.

If the deleted branch had children you want to keep stacked, reparent them first (replace `<name>` with the deleted branch name):

```text
sm untrack <name> --reparent
```

If you don't care about children, drop the git config section directly:

```text
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

Open an issue on [github.com/bluegardenproject/stac-man](https://github.com/bluegardenproject/stac-man/issues) with the output of:

```bash
sm version
sm doctor
git config --list --local | grep stac-man
```
