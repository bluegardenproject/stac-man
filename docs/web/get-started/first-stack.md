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

Branch off and record `main` as the parent:

```bash
sm create feat/auth-models
```

Edit the files for this branch (e.g. `internal/models/user.go`) in your editor. Once the working tree has the changes you want, commit and restack descendants in one step:

```bash
sm modify -a -m "models: add User and Session"
```

What just happened:

- `sm create` branched off `main` and recorded `main` as the parent.
- `sm modify -a -m` staged tracked-but-modified files, committed them, and restacked any descendants. (No descendants yet, so nothing to restack.)

## 3. Stack the second branch

Branch off the current branch (so the new branch's parent is `feat/auth-models`):

```bash
sm create feat/auth-handlers
```

Edit the files for this branch (e.g. `internal/handlers/login.go`), then commit:

```bash
sm modify -a -m "handlers: implement /login"
```

Now `sm log` shows the chain:

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

Switch to the bottom branch:

```bash
sm checkout feat/auth-models
```

Edit the files you want to change (e.g. `internal/models/user.go`), then commit:

```bash
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
sm status
sm checkout feat/auth-handlers
sm land
```

`sm land` finds the bottom-most PR in your path (`feat/auth-models`), merges it through `gh pr merge` (squash by default), and runs `sm sync` so the merged branch is deleted locally and `feat/auth-handlers` is re-parented onto trunk.

Your tree now looks like:

```
stac-man

main
└─ feat/auth-handlers  ← current  #2 open
```

The PR #2 stack table and cached GitHub metadata are refreshed in place — it no longer mentions the merged `feat/auth-models` row.

## What to read next

- [Concepts](/concepts/) — the mental model behind these verbs.
- [Recipes](/recipes/) — multi-step workflows like splitting a fat branch or recovering from a bad rebase.
- [Command reference](/commands/) — every flag, every command.
