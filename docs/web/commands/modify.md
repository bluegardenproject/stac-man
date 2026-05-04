# sm modify

The everyday "commit + restack" shortcut. Defaults to amending the current commit and rebases every descendant so the chain stays consistent.

## Synopsis

```
sm modify [-c] [--amend] [-a] [--include-untracked] [-m <msg>]
```

## Flags

| Flag | Description |
|---|---|
| `-c`, `--commit` | Create a new commit instead of amending. |
| `--amend` | Amend the current commit (default; flips off when `-c` is set without `--amend`). |
| `-a`, `--all` | Stage tracked-but-modified files before committing (mirrors `git commit -a`). |
| `--include-untracked` | With `-a`, also stage new (untracked) files. |
| `-m`, `--message <msg>` | Commit message (required with `-c`). |

## Examples

Default — amend in place and restack descendants. Make whatever edits you want in your editor first, then run:

```bash
sm modify -a
```

Add a new commit on the current branch and restack:

```bash
sm modify -c -a -m "models: add password hash"
```

Amend with a fresh message:

```bash
sm modify -a -m "models: add User and Session"
```

## What it does

1. Stages files (if `-a`).
2. Either amends `HEAD` or creates a new commit on the current branch.
3. Walks every tracked descendant in topological order and rebases each onto its parent's new tip.
4. On conflict, returns a [`PausedError`](/concepts/restack#pause-and-resume) — resolve, `git add`, then run [`sm continue`](./continue).

## See also

- [`sm restack`](./restack) — explicit form when you've changed history with plain git.
- [`sm absorb`](./absorb) — route uncommitted hunks into ancestor commits instead of always landing them on the current tip.
- [`sm split`](./split) — decompose a fat commit list into smaller branches.
