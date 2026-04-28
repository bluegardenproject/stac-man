# Absorb fixups

You're three branches deep in a stack and you spot a typo (or a missed null-check) in commit 4. You want the fix to land in commit 4, not as a fresh commit on the current tip.

## What you'd otherwise do

```bash
git commit --fixup=<sha-of-commit-4>
git rebase -i --autosquash main
sm restack
```

Three commands, the middle one drops you into an interactive editor. Repeat per hunk if multiple commits are involved.

## With sm absorb

[`sm absorb`](/commands/absorb) wraps [`git-absorb`](https://github.com/tummychow/git-absorb), which matches each hunk to the commit that introduced the surrounding code:

```bash
sm absorb
```

What happens:

1. `git-absorb` looks at every hunk in the working tree.
2. For each hunk, it `git blame`s the surrounding lines to find the right ancestor commit.
3. It creates `fixup!` commits and runs `git rebase --autosquash` to fold them in.
4. `sm` restacks every descendant so they pick up the new commits.

End result: one command, no editor, descendants are consistent.

## Prerequisites

Install `git-absorb`:

```bash
brew install git-absorb              # macOS
cargo install git-absorb              # any platform with rustup
```

If `git-absorb` is missing, `sm absorb` prints an install hint and exits. It's not bundled with `sm`.

## Worked example

Starting state:

```
main
└── feat/api-models
└── feat/api-handlers
└── feat/api-tests   (current, working-tree dirty with a fix)
```

The dirty hunk lives in `internal/handlers/login.go` — code that was originally written in `feat/api-handlers`'s second commit. Run:

```bash
sm absorb
```

Output:

```
✓ absorbed into ancestors of feat/api-tests (base: main)
  descendants restacked
```

Under the hood:

- The fixup landed on the right commit in `feat/api-handlers`.
- `feat/api-tests` was rebased onto the rewritten `feat/api-handlers`.
- Your working tree is clean.

## Picking a base

If you want absorb to consider only a portion of the stack, pass `--base`:

```bash
sm absorb --base feat/api-handlers
```

Now `git-absorb` will not consider commits below `feat/api-handlers` as fixup targets.

## When absorb can't help

`git-absorb` works on hunks, not whole files. If your fix touches code that was added across multiple commits, absorb may decline to find a single target — it will print which hunks were absorbed and which were left behind.

For those leftover hunks, the fallback is to commit them on the current tip with `sm modify`, or to use `git commit --fixup=<sha>` manually.

## On conflict

If restacking descendants conflicts after the absorb, the standard pause protocol kicks in:

```
⚠ rebase paused on feat/api-tests
```

Resolve, `git add`, [`sm continue`](/commands/continue).

## See also

- [`sm absorb`](/commands/absorb) — flag reference.
- [`sm modify`](/commands/modify) — when the fix belongs on the current tip.
- [git-absorb upstream](https://github.com/tummychow/git-absorb) — the underlying tool.
