# Trunk

**Trunk** is the long-lived branch every stack ultimately rebases onto — usually `main` or `master`. `sm` needs to know which branch that is so it can render the tree, prune merged work, and pick the right base when no parent is recorded.

## Auto-detection

On the first `sm` command in a fresh repo, trunk is detected in this order:

1. The branch pointed to by `origin/HEAD` (what `git remote set-head origin --auto` produces).
2. `main`, if it exists locally.
3. `master`, if it exists locally.

The result is persisted to `.git/config`:

```ini
[stac-man]
    trunk = main
```

After the first run, `sm` reads this key directly — no detection cost.

## Overriding

If your team's trunk is something else (`develop`, `trunk`, …), set it manually:

```bash
git config --local stac-man.trunk develop
```

That's the only way to change it. `sm` deliberately doesn't expose a flag for this — trunk is a per-repo setting that should rarely change.

## What "trunk" means in each command

| Command | Trunk's role |
|---|---|
| `sm create` | Default parent for a new branch made off trunk. |
| `sm log` | Root of the rendered tree. |
| `sm restack` | Stops walking up at trunk; trunk itself is never rewritten. |
| `sm sync` | Fast-forwarded against `origin/<trunk>`; merged stack-mates are deleted; survivors restacked. |
| `sm submit` | Default base for branches whose parent IS trunk. |
| `sm doctor` | Reports trunk + tracked-branch count first; flags branches whose parent isn't reachable from trunk. |

## Trunk vs. parent

Trunk is one specific branch in the repo. **Parent** is per-branch metadata — every tracked branch has a parent (which may or may not be trunk). A branch sitting directly on trunk has parent = trunk; a branch deeper in the stack has parent = some other tracked branch.

## What trunk is NOT

- Not necessarily the default branch on GitHub. They line up by convention but `sm` reads its own `stac-man.trunk` key, not GitHub's default.
- Not protected by `sm`. If you want force-push protection on trunk, that's a `git push` setting / GitHub branch protection rule. `sm` will not push to trunk on your behalf.
- Not a "stack" of size one. Trunk is the floor of every stack, not part of it.
