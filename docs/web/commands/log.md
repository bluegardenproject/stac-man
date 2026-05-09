# sm log

Print the stack tree rooted at trunk with current-branch / needs-restack markers and cached PR summaries. `sm log` is local-first in v2: it does not fetch live GitHub checks or mergeability.

Alias: `sm ls`.

## Synopsis

```
sm log [--no-pr] [--json | --porcelain]
```

## Flags

| Flag | Description |
|---|---|
| `--no-pr` | Skip cached PR decoration entirely. The stack shape still renders from local metadata. |
| `--json` | Emit the stack as a single JSON document on stdout instead of the rendered tree. The per-branch shape matches `sm show --json` so a single parser handles both commands. Mutually exclusive with `--porcelain`. |
| `--porcelain` | Emit one tab-separated row per tracked branch with no header. Stable column order; empty fields written as `-`. Mutually exclusive with `--json`. |

## Example output

```
stac-man

main
└─ feat/db-schema  #41 open
   └─ feat/api-endpoints  #42 open
      └─ feat/web-form  ← current  #43 open
         └─ feat/web-tests  #44 draft
```

What each column means:

- **Branch name.** Trunk is at the top with no connector. Children sit under their parents. The branch HEAD is on now is tagged `← current`. A branch whose recorded parent SHA no longer matches the parent's tip gets `(needs restack)`.
- **PR pill.** `#N <state>`: `open`, `draft`, `merged`, or `closed`. Branches with no cached PR summary show no pill until `sm submit` or `sm sync` has populated the local cache.
- **Live GitHub status.** CI and mergeability are intentionally not fetched by `sm log`; use [`sm status`](./status) when you need the latest GitHub answer.

## What it does

1. Loads the in-memory stack `Graph` from `.git/config`.
2. Reads cached PR summaries from `.git/stac-man/cache.db`.
3. Renders the tree, top-down, with one branch per row.

## Cache behavior

GitHub-derived data is cached in `.git/stac-man/cache.db`, under the real git directory returned by `git rev-parse --git-dir`. The database is local to the repository and disposable: git config remains the source of truth for stack relationships.

`sm log` trusts the cache so it can stay fast and work offline. Use:

- [`sm status`](./status) to fetch fresh checks and mergeability for tracked PRs.
- [`sm sync`](./sync) to refresh the repository and cached GitHub state together.
- `sm submit` to opportunistically update PR summaries for PRs it creates or updates.

## Scripting (`--json` and `--porcelain`)

Both formats consume exactly the same local-first graph as the rendered tree. `--no-pr` leaves cached PR fields empty while still reporting the local branch graph.

### JSON

```bash
sm log --json
```

Top-level shape:

```json
{
  "trunk": "main",
  "current": "feat-web-form",
  "branches": [
    {
      "branch": "feat-db-schema",
      "parent": "main",
      "parentSHA": "abc123…",
      "depth": 1,
      "children": ["feat-api-endpoints"],
      "pr": {
        "number": 41,
        "state": "OPEN",
        "url": "https://github.com/org/repo/pull/41",
        "draft": false,
        "title": "Add db schema",
        "title": "Add db schema"
      }
    },
    { "branch": "feat-api-endpoints", "...": "..." }
  ]
}
```

Notes:

- The trunk is reported once at the top level. **Branches never contains a trunk row** — every entry is a tracked (non-trunk) branch.
- Order is depth-first pre-order over name-sorted roots, so a parent always appears before its descendants and the same input graph always produces the same output.
- `pr` is `null` (omitted) when there's no recorded PR. When `--no-pr` is set or `gh` is unavailable, branches with a recorded number still surface a minimal `{ "number": N }` so consumers see the link.
- `pr.checks` and `pr.mergeable` are omitted from default `sm log` output because live GitHub status is owned by `sm status`.
- Boolean fields (`isCurrent`, `needsRestack`, `pr.draft`) are omitted when `false`.

A few `jq` recipes:

```bash
sm log --json | jq -r '.branches[] | select(.pr.state == "OPEN") | .branch'
sm log --json | jq -r '.branches[] | select(.needsRestack) | .branch'
```

### Porcelain

```bash
sm log --porcelain
```

One tab-separated row per tracked branch. No header. Empty fields are written as `-`. Booleans are `true` / `false`.

Column order (stable across releases — new columns are appended, never reordered):

| # | Column | Values |
|---|---|---|
| 1 | `branch` | branch name |
| 2 | `parent` | parent branch name (`-` if none) |
| 3 | `depth` | integer; `1` for branches rooted on trunk |
| 4 | `pr_number` | integer or `-` |
| 5 | `pr_state` | `open` / `draft` / `merged` / `closed` / `-` |
| 6 | `ci` | Always `-` in default v2 output; live status is fetched by `sm status`. |
| 7 | `mergeable` | Always `-` in default v2 output; live status is fetched by `sm status`. |
| 8 | `is_current` | `true` / `false` |
| 9 | `needs_restack` | `true` / `false` |

Example:

```
feat-db-schema     main             1  41  open   -        -            false  false
feat-api-endpoints feat-db-schema   2  42  open   -        -            false  false
feat-web-form      feat-api-endpoints 3 43  open   -        -            true   false
feat-web-tests     feat-web-form    4  44  draft  -        -            false  false
```

(In real output the columns are tab-separated; the spacing above is for readability.)

`--porcelain` is the format to reach for from shell pipelines. `cut -f1,9` lists branches that need a restack.

## See also

- [`sm show`](./show) — detailed view of one branch, including its commits. The `pr` JSON shape is shared with `sm log --json`.
- [`sm status`](./status) — fetch live GitHub checks and mergeability.
- [`sm doctor`](./doctor) — check local metadata when something looks off in `sm log`.
