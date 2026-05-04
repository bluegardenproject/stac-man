# Split a fat branch

You started with one branch, kept committing, and ended up with seven commits that really should be three reviewable PRs. [`sm split`](/commands/split) is the verb.

## Starting state

```
main
└── feat/auth-everything   (current)
       7 commits:
         1. models: add User
         2. models: add Session
         3. models: tests
         4. handlers: /login
         5. handlers: /logout
         6. handlers: tests
         7. docs: auth README
```

You want three branches:

```
main
└── feat/auth-models   (commits 1-3)
    └── feat/auth-handlers   (commits 4-6)
        └── feat/auth-docs   (commit 7)
```

## Step 1 — see the commit indices

```bash
sm show
```

The `Commits:` section lists them 1-based, oldest first. (You can also use `git log --reverse --oneline main..HEAD` if you prefer.)

## Step 2 — split with explicit ranges

```bash
sm split \
  --names auth-models,auth-handlers,auth-docs \
  --commits "1-3,4-6,7"
```

Output:

```
✓ split feat/auth-everything into 3 branch(es):
  feat/auth-models
  feat/auth-handlers
  feat/auth-docs
```

After this, `feat/auth-everything` is replaced by the chain above. HEAD is on the topmost new branch (`feat/auth-docs`). `sm log` confirms:

```
stac-man

main
└─ feat/auth-models
   └─ feat/auth-handlers
      └─ feat/auth-docs  ← current
```

## Step 3 — submit as a stack

```bash
sm submit --stack
```

Three PRs, bases wired correctly:

| PR | Branch | Base |
|---|---|---|
| #1 | `feat/auth-models` | `main` |
| #2 | `feat/auth-handlers` | `feat/auth-models` |
| #3 | `feat/auth-docs` | `feat/auth-handlers` |

## Variations

### Auto split (one branch per commit)

If the commit subjects are already meaningful, drop the flags:

```bash
sm split
```

Each commit becomes a branch named after its slugified subject (`models-add-user`, `models-add-session`, …). Useful for small chains; rename afterward if you don't like the auto names.

### Per-hunk split

Not supported. If you committed two unrelated changes in one commit, undo the commit first, then re-stage and re-commit each chunk separately. Substitute real commit messages for `"<message-a>"` / `"<message-b>"`:

```text
git reset --soft HEAD^
git add -p     # interactively pick hunks for commit A
git commit -m "<message-a>"
git add .      # the rest go into commit B
git commit -m "<message-b>"
sm split --names a,b --commits "1,2"
```

## Recovery

Don't like the result? [`sm undo`](/commands/undo) rewinds the split:

```bash
sm undo
```

Your original `feat/auth-everything` comes back with all seven commits.

## See also

- [`sm split`](/commands/split) — flag reference.
- [`sm fold`](/commands/fold) — the inverse direction.
- [Recipes → Absorb fixups](./absorb-fixups) — for the "I added one fixup commit and want it in commit 3" version of this problem.
