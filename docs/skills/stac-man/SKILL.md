---
description: Use the stac-man (sm) CLI to manage stacked branches and pull requests. Covers when to reach for sm vs plain git, the canonical workflow (create → modify → restack → submit), what each command does, and the gotchas that catch first-time users.
---

# stac-man — using the `sm` CLI

`stac-man` (binary: `sm`) is a CLI for managing **stacked pull requests** the way Graphite's `gt` does, but free, local-only, and without the IDE plugin. Stack metadata lives in the local git config; pull-request operations are delegated to `gh`.

This skill applies whenever the user wants to:

- Build a stack of dependent branches/PRs.
- Restack a chain after an upstream change.
- Submit / update a stack of PRs on GitHub with the right base branches wired.
- Inspect the stack with `sm log`.

## Prerequisites (verify once per repo)

```bash
sm --version       # confirms sm is installed
gh auth status     # confirms gh is authenticated (only needed for sm submit)
```

If `sm` isn't installed: tell the user to install it from `github.com/philipptpunkt/stac-man` (binary) and re-run.

`sm` auto-detects the trunk on first invocation (origin/HEAD → main → master) and persists it to `git config --local stac-man.trunk`. No setup step is required.

## The canonical workflow

```bash
# 1. Start a feature stack from trunk
git switch main
sm create feat/auth-models     # branches off main, parent=main

# 2. Make changes, commit
$EDITOR …
sm modify -a -m "models: add User and Session"

# 3. Stack a second branch on top
sm create feat/auth-handlers
$EDITOR …
sm modify -a -m "handlers: implement /login"

# 4. Submit the whole stack as PRs
sm submit --stack
```

## Commands at a glance

| Command | What it does |
|---|---|
| `sm create <name>` | Creates a new branch off HEAD; records the previous branch as its parent. With `-m` and `-a` it commits in one step. |
| `sm modify [-c] [-a] [-m msg]` | Default: amends the current commit. With `-c` creates a new commit. With `-a` stages all changes first. **Always restacks descendants automatically.** |
| `sm log` (alias: `ls`) | Renders the stack tree from trunk down, with current-branch / needs-restack / PR markers. |
| `sm checkout [name]` (alias: `co`) | Switch HEAD to a tracked branch. Without arg, lists choices. |
| `sm up` / `sm down` / `sm top` / `sm bottom` | Walk the stack relative to current. `--first` resolves forks alphabetically. |
| `sm restack [branch]` | Rebases the chain rooted at branch onto current parent tips. Pauses on conflict. |
| `sm continue` / `sm abort` | Resume or bail out of a paused restack/sync. |
| `sm sync` | Fetch, fast-forward trunk, delete merged branches, restack survivors. |
| `sm submit [--stack] [--draft]` | Push branch(es) and open or update PRs via `gh`, wiring base branches correctly. |
| `sm track [branch] [--parent X]` / `sm untrack [--reparent]` | Adopt or forget existing branches into/out of the stack graph. |
| `sm parent [--set X]` / `sm children` | Inspect or reassign parent/child relationships. |
| `sm fold [-m msg]` | Squash the current branch into its parent and re-parent any children. |
| `sm doctor` (alias: `status`) | Sanity-check stac-man metadata vs. git state. Print first whenever the user reports something weird. |

## Decision rules for the agent

- **Branching off another branch:** prefer `sm create <name>` over `git switch -c <name>` so the parent gets recorded.
- **Amending mid-stack:** prefer `sm modify --amend` over `git commit --amend`; the former auto-restacks every descendant.
- **Pulling latest trunk:** prefer `sm sync` over `git pull`; sync also deletes merged branches and restacks the rest.
- **Pushing PRs:** prefer `sm submit --stack` over running `gh pr create` per branch; submit retargets bases when the stack changed.
- **User says "rebase"**: in a stack, that almost always means `sm restack`, not `git rebase`. Use `sm restack`.
- **User says "switch branches"**: use `sm checkout <name>` (or `sm up` / `sm down`) so navigation feels stack-aware.
- **Things look broken (stale parent SHA, orphan branches, untracked roots):** run `sm doctor` first; it tells you exactly what's drifted.

## Conflict handling

When `sm restack` (or `sm modify`, `sm sync`, `sm submit` after restack) hits a conflict:

1. The CLI prints `⚠ rebase paused on <branch>`.
2. The user resolves the conflict in files, runs `git add <resolved>`.
3. Run `sm continue` to resume the walk. It will continue rebasing the rest of the stack.
4. If the user wants to bail out, `sm abort` discards the in-progress rebase and restores the original branch.

The agent should NOT call `git rebase --continue` directly — `sm continue` does that AND processes the remaining queue.

## State storage

- Per-repo: stack metadata lives in `.git/config` under `branch.<name>.stac-man-parent`, `branch.<name>.stac-man-parent-sha`, `branch.<name>.stac-man-pr`, and `stac-man.trunk`.
- Per-user (optional): `~/.config/stac-man/config.yaml` for preferences only (color, default base, draft-PR default). Missing file is fine — `sm` works with built-in defaults.
- Resume state during paused rebase: `.git/stac-man/restack.json` (auto-managed; user shouldn't touch).

## NEVER

- Don't call `git rebase` or `git rebase --continue` directly when the user is in a stack — use `sm restack` and `sm continue`.
- Don't manually edit `.git/stac-man/restack.json`. Use `sm continue` / `sm abort`.
- Don't `git branch -D` a tracked branch without `sm untrack` first; the metadata will dangle. (`sm doctor` will catch it after the fact.)
- Don't push branches with `git push -f`; use `sm submit` so the force is `--force-with-lease`.
- Don't add AI-tool attribution to commit messages.
