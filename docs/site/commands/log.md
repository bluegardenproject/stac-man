# sm log

Print the stack tree rooted at trunk, with current-branch / needs-restack markers and PR status.

Alias: `sm ls`.

## Synopsis

```
sm log [--no-pr]
```

## Flags

| Flag | Description |
|---|---|
| `--no-pr` | Skip the PR status lookup (no `gh` calls). |

## Example output

```
* feat/web-form               #43 OPEN
  └─ feat/api-endpoints       #42 OPEN
     └─ feat/db-schema        #41 DRAFT
        └─ main
```

The active branch is marked with `*`. PR pills show `OPEN` / `DRAFT` / `MERGED` / `CLOSED`. Branches that need a restack are highlighted (the colored output is shown in your terminal).

## What it does

1. Loads the in-memory stack `Graph` from `.git/config`.
2. For every tracked branch with a recorded PR number, calls `gh pr view --json` to fetch state (skipped with `--no-pr`).
3. Renders the tree, top-down, with one branch per row.

## When to use `--no-pr`

- You're offline.
- `gh` isn't installed or authenticated.
- You want the tree to render instantly (no network round-trips).

## See also

- [`sm show`](./show) — detailed view of one branch.
- [`sm doctor`](./doctor) — when something looks off in `sm log`.
