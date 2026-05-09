> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm show

Detailed view of a single branch: parent, children, ahead/behind counts, PR state, and the commits unique to it.

## Synopsis

```
sm show [branch] [--json]
```

If no branch is given, shows the current branch.

## Flags

| Flag | Description |
|---|---|
| `--json` | Emit a machine-readable `BranchView` instead of the human summary. |

## Example output

```
feat/auth-handlers

  Parent: feat/auth-models
  Status: up to date
  Trunk:  main
  Tip:    8a4f3c1
  vs feat/auth-models: +3 -0
  vs main:             +5 -0
  Ancestors: feat/auth-models → main
  PR: #42 OPEN https://github.com/owner/repo/pull/42
  Title: handlers: implement /login

  Commits:
     1. 8a4f3c1 handlers: implement /login
     2. 3e2c1f4 handlers: validate body
     3. b1d8e0a handlers: tests
```

## --json

```bash
sm show feat/auth-handlers --json
```

Emits a `BranchView` object with the same fields, suitable for `jq` or for AI agents reading branch state programmatically. Prefer `--json` over scraping the human form.

## What it does

1. Loads the in-memory `Graph` and locates the branch.
2. Computes ahead/behind vs. parent and trunk via `git rev-list`.
3. Looks up PR state via `gh pr view --json` (best-effort; failures don't block).
4. Lists the commits unique to the branch.

## See also

- [`sm log`](./log) — every branch in one tree.
- [`sm parent`](./parent) / [`sm children`](./children) — narrower one-liners.
- [Concepts → Parent metadata](/v1/concepts/parent-metadata).
