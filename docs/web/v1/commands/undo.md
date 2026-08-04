> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm undo

Roll back the most recent stac-man operation.

## Synopsis

```
sm undo [--dry-run]
```

## Flags

| Flag | Description |
|---|---|
| `--dry-run` | Print what would be reversed without changing anything. |

## Examples

Preview before committing to it:

```bash
sm undo --dry-run
# would undo modify (auth-models) — branches: feat/auth-models, feat/auth-handlers
```

Actually undo:

```bash
sm undo
# ✓ undid modify (auth-models)
#   branches: feat/auth-models, feat/auth-handlers
```

## What it does

1. Pops the latest entry from `.git/stac-man/history.json` (newest-first, capped at 50 entries).
2. Restores every branch the entry touched — tip SHA, parent metadata, tracking state.

The restoration is best-effort and uses git's reflog for branch tips, so even branches that were deleted as part of the original op can usually be brought back.

## What gets recorded

Every mutating `sm` command records a history entry before its first mutation:

`create`, `modify`, `restack`, `sync`, `submit`, `absorb`, `move`, `parent --set`, `track`, `untrack`, `fold`, `split`, `land`, `get` …

Read-only commands (`log`, `show`, `parent`, `children`, `doctor`) do not.

## What it can't undo

- A merge that already happened on GitHub. `sm undo` is a local rewind; it doesn't talk to `gh`.
- Operations older than the 50-entry cap.
- Operations from before `sm undo` was added (any pre-v2 history is empty).

## When it refuses

`sm undo` refuses while a restack is paused. Finish or abort the paused operation first ([`sm continue`](./continue) or [`sm abort`](./continue#sm-abort)).

## See also

- [`sm doctor`](./doctor) — verify the chain after an undo.
- [Concepts → Parent metadata](/v1/concepts/parent-metadata) — what gets stored.
