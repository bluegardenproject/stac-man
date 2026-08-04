> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm create

Create a new branch off HEAD and record the current branch as its parent.

## Synopsis

```
sm create <branch> [-m <msg>] [-a] [--include-untracked]
```

## Flags

| Flag | Description |
|---|---|
| `-m`, `--message <msg>` | Commit message for an initial commit on the new branch. |
| `-a`, `--all` | Stage tracked-but-modified files before committing (mirrors `git commit -a`). |
| `--include-untracked` | With `-a`, also stage new (untracked) files. |

## Examples

Branch off and start dirty — switch to trunk first, then create the branch:

```bash
git switch main
sm create feat/auth-models
```

Edit the files you want, then commit and restack with one call:

```bash
sm modify -a -m "models: add User"
```

Branch off AND commit in one step (the changes are already in your working tree):

```bash
sm create feat/auth-models -a -m "models: add User"
```

Pull in untracked new files at the same time:

```bash
sm create feat/auth-models -a --include-untracked -m "models: add User"
```

## What it does

1. Branches off HEAD with the given name.
2. Records the previous branch as the new branch's parent (`branch.<n>.stac-man-parent`).
3. With `-m` (and optionally `-a`), creates a commit on the new branch.

The parent recording is the whole point — without it, every other `sm` command treats your new branch as untracked.

## See also

- [`sm modify`](./modify) — commit + restack.
- [`sm track`](./track) — adopt a branch you created with plain `git switch -c`.
