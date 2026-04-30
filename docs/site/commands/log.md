# sm log

Print the stack tree rooted at trunk, with current-branch / needs-restack markers, PR pills, and per-row CI / mergeability badges.

Alias: `sm ls`.

## Synopsis

```
sm log [--no-pr] [--no-checks] [--no-merge-status] [--json | --porcelain]
```

## Flags

| Flag | Description |
|---|---|
| `--no-pr` | Skip the PR status lookup entirely (no `gh` calls). Disables both badges as well, since they share the same round-trip. |
| `--no-checks` | Skip the CI rollup badge per row. PR pills still render. |
| `--no-merge-status` | Skip the GitHub mergeability badge per row. PR pills and CI badges still render. |
| `--json` | Emit the stack as a single JSON document on stdout instead of the rendered tree. The per-branch shape matches `sm show --json` so a single parser handles both commands. Mutually exclusive with `--porcelain`. |
| `--porcelain` | Emit one tab-separated row per tracked branch with no header. Stable column order; empty fields written as `-`. Mutually exclusive with `--json`. |

## Example output

```
stac-man

main
└─ feat/db-schema  #41 open CI ready
   └─ feat/api-endpoints  #42 open CI ready
      └─ feat/web-form  ← current  #43 open CI conflict
         └─ feat/web-tests  #44 draft CI
```

What each column means:

- **Branch name.** Trunk is at the top with no connector. Children sit under their parents. The branch HEAD is on now is tagged `← current`. A branch whose recorded parent SHA no longer matches the parent's tip gets `(needs restack)`.
- **PR pill.** `#N <state>`: `open`, `draft`, `merged`, or `closed`. Branches with no recorded PR show no pill.
- **CI badge.** A coloured `CI` chip from the PR's check rollup: green when every check passed, yellow when at least one is still in flight, red when at least one failed. Hidden entirely when the PR has no checks configured.
- **Mergeability badge.** A green `ready` chip when GitHub reports the PR as `MERGEABLE`; an orange `conflict` chip when `CONFLICTING`. Hidden on drafts and on PRs whose state GitHub is still computing — a missing badge does not mean "fine".

## What it does

1. Loads the in-memory stack `Graph` from `.git/config`.
2. For every tracked branch with a recorded PR number, calls `gh pr list --head <branch>` to fetch PR state for the pill.
3. For every PR whose state is `open` (including drafts), issues one `gh pr view --json mergeable,state,isDraft,statusCheckRollup` to populate the CI and mergeability badges.
4. Renders the tree, top-down, with one branch per row.

## Caching

The combined CI + mergeability response is cached for 60 seconds at `.git/stac-man/checks-cache.json`. Repeated `sm log` calls within that window render instantly. The cache is invalidated automatically after `sm submit` and `sm sync`, both of which can change CI state and mergeability — the very next `sm log` will re-fetch.

## When to use the opt-outs

- `--no-pr` — you're offline, `gh` isn't installed or authenticated, or you want the tree to render with zero network round-trips.
- `--no-checks` — your stack has noisy CI matrices and you only care about the merge status.
- `--no-merge-status` — GitHub's mergeability check is currently flaky for your repo and the badges are misleading.

## Scripting (`--json` and `--porcelain`)

Both formats consume exactly the same underlying graph as the rendered tree, including the `--no-pr` / `--no-checks` / `--no-merge-status` opt-outs — the same options that suppress a badge in the tree leave the equivalent JSON / porcelain field empty (or unset).

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
        "checks": "PASS",
        "mergeable": "MERGEABLE"
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
- `pr.checks` and `pr.mergeable` are omitted when not computed; their values are the constants `PASS` / `PENDING` / `FAIL` and `MERGEABLE` / `CONFLICTING` / `UNKNOWN` respectively.
- Boolean fields (`isCurrent`, `needsRestack`, `pr.draft`) are omitted when `false`.

A few `jq` recipes:

```bash
sm log --json | jq -r '.branches[] | select(.pr.checks == "FAIL") | .branch'
sm log --json | jq '.branches | map(select(.pr.mergeable == "CONFLICTING")) | length'
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
| 6 | `ci` | `pass` / `pending` / `fail` / `-` |
| 7 | `mergeable` | `mergeable` / `conflicting` / `-` |
| 8 | `is_current` | `true` / `false` |
| 9 | `needs_restack` | `true` / `false` |

Example:

```
feat-db-schema     main             1  41  open   pass     mergeable    false  false
feat-api-endpoints feat-db-schema   2  42  open   pass     mergeable    false  false
feat-web-form      feat-api-endpoints 3 43  open   pass     conflicting  true   false
feat-web-tests     feat-web-form    4  44  draft  -        -            false  false
```

(In real output the columns are tab-separated; the spacing above is for readability.)

`--porcelain` is the format to reach for from shell pipelines. `awk -F'\t' '$6 == "fail"'` listed every branch with red CI; `cut -f1,9` lists branches that need a restack.

## See also

- [`sm show`](./show) — detailed view of one branch, including its commits. The `pr` JSON shape is shared with `sm log --json`.
- [`sm doctor`](./doctor) — when something looks off in `sm log`. Doctor surfaces a louder block for PRs GitHub reports as `CONFLICTING`.
