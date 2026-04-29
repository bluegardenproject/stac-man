# Your first stack

Five minutes, two branches, one PR stack. We'll build:

```
main
└── feat/auth-models       (PR #1)
    └── feat/auth-handlers (PR #2 — base = feat/auth-models)
```

This walkthrough assumes you already have [`sm` installed](./install) and [`gh` authenticated](https://cli.github.com/).

## 1. Start from trunk

```bash
git switch main
```

`sm` auto-detects your trunk on first invocation (origin/HEAD → main → master) and persists it to `git config --local stac-man.trunk`. No setup step is required.

## 2. Create the bottom branch

```bash
sm create feat/auth-models
$EDITOR internal/models/user.go
sm modify -a -m "models: add User and Session"
```

What just happened:

- `sm create` branched off `main` and recorded `main` as the parent.
- `sm modify -a -m` staged tracked-but-modified files, committed them, and restacked any descendants. (No descendants yet, so nothing to restack.)

## 3. Stack the second branch

```bash
sm create feat/auth-handlers
$EDITOR internal/handlers/login.go
sm modify -a -m "handlers: implement /login"
```

`sm create` again branches off the current branch, so `feat/auth-handlers`'s parent is `feat/auth-models`. Now `sm log` shows the chain:

```
stac-man

main
└─ feat/auth-models
   └─ feat/auth-handlers  ← current
```

## 4. Submit as PRs

```bash
sm submit --stack
```

This pushes both branches and opens two PRs through `gh`:

- PR #1 — `feat/auth-models` → `main`
- PR #2 — `feat/auth-handlers` → `feat/auth-models`

Re-running `sm submit --stack` later is idempotent: branches that match origin are skipped, existing PRs are retargeted if their parent changed locally.

## 5. Iterate

Make a change to the bottom branch:

```bash
sm checkout feat/auth-models
$EDITOR internal/models/user.go
sm modify -a -m "models: add password hash"
```

Because `sm modify` always restacks descendants, `feat/auth-handlers` is automatically rebased onto the new tip of `feat/auth-models`. No manual `git rebase` needed.

Push the update:

```bash
sm submit --stack
```

## 6. Land the bottom

When PR #1 has approvals and a green CI:

```bash
sm checkout feat/auth-handlers
sm land
```

`sm land` finds the bottom-most PR in your path (`feat/auth-models`), merges it through `gh pr merge` (squash by default), and runs `sm sync` so the merged branch is deleted locally and `feat/auth-handlers` is re-parented onto trunk.

Your tree now looks like:

```
stac-man

main
└─ feat/auth-handlers  ← current  #2 open  CI ready
```

The PR #2 stack table is also refreshed in place — it no longer mentions the merged `feat/auth-models` row.

## What to read next

- [Concepts](/concepts/) — the mental model behind these verbs.
- [Recipes](/recipes/) — multi-step workflows like splitting a fat branch or recovering from a bad rebase.
- [Command reference](/commands/) — every flag, every command.
