# sm status

Fetch live GitHub checks and mergeability for tracked PRs, then write the result into the local SQLite cache.

## Synopsis

```
sm status
```

No flags.

## Example output

```
stac-man status

  → main (trunk)

  open #41 feat/db-schema CI pass mergeable
  open #42 feat/api-endpoints CI pending
  draft #43 feat/web-form CI fail
```

## What it does

1. Loads the local stack graph from `.git/config`.
2. Finds tracked branches with recorded PR numbers.
3. Fetches each PR's current state, draft flag, check rollup, and mergeability from GitHub through `gh`.
4. Stores the result in `.git/stac-man/cache.db`.
5. Prints the live result.

Rows that fail to fetch are shown as errors, but the command keeps going so the rest of the stack can still update.

## When to use it

Use `sm status` when you need GitHub's current answer. `sm log` stays local-first and does not block on live checks or mergeability.

For a full repository refresh, run [`sm sync`](./sync). Sync fetches from origin, cleans up merged branches, restacks survivors, retargets PRs, and refreshes cached GitHub status as part of the cleanup pass.

## See also

- [`sm log`](./log) — fast local stack view with cached PR summaries.
- [`sm sync`](./sync) — refresh local git state and cached GitHub state together.
- [`sm doctor`](./doctor) — local-only metadata health checks.
