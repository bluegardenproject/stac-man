# sm log

Print the stack tree rooted at trunk, with current-branch / needs-restack markers, PR pills, and per-row CI / mergeability badges.

Alias: `sm ls`.

## Synopsis

```
sm log [--no-pr] [--no-checks] [--no-merge-status]
```

## Flags

| Flag | Description |
|---|---|
| `--no-pr` | Skip the PR status lookup entirely (no `gh` calls). Disables both badges as well, since they share the same round-trip. |
| `--no-checks` | Skip the CI rollup badge per row. PR pills still render. |
| `--no-merge-status` | Skip the GitHub mergeability badge per row. PR pills and CI badges still render. |

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

## See also

- [`sm show`](./show) — detailed view of one branch, including its commits.
- [`sm doctor`](./doctor) — when something looks off in `sm log`. Doctor surfaces a louder block for PRs GitHub reports as `CONFLICTING`.
