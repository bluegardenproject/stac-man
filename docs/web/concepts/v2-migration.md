# Migrating to v2

v2 makes inspection commands local-first. The goal is Graphite-style responsiveness without adding a service: stack relationships still live in local git config, while GitHub-derived PR/status data is cached in `.git/stac-man/cache.db`.

## What changed

| v1 behavior | v2 behavior |
|---|---|
| `sm log` could fetch PR state, CI, and mergeability from GitHub. | `sm log` renders local stack shape plus cached PR summaries only. |
| `sm doctor` also had the `sm status` alias and could surface GitHub merge conflicts. | `sm doctor` is local metadata health only. |
| CI/mergeability used a short-lived JSON cache. | Live status is fetched by `sm status` and stored in SQLite. |
| The cockpit could show empty/loading status while refreshing. | The cockpit renders cached status immediately and refreshes in the background. |

## New command split

- Use [`sm log`](/commands/log) for the fast local stack view.
- Use [`sm status`](/commands/status) when you need live GitHub checks and mergeability.
- Use [`sm sync`](/commands/sync) when you want the repository and GitHub cache brought to the latest known state together.
- Use [`sm doctor`](/commands/doctor) for local metadata drift, missing branches, and restack health.

## Cache location

The SQLite database lives under the real git directory:

```text
<git-dir>/stac-man/cache.db
```

In a normal clone this is `.git/stac-man/cache.db`. In worktrees or submodules, `sm` follows `git rev-parse --git-dir` so the cache stays attached to the right repository metadata area. The cache is disposable; deleting it only removes derived GitHub data.

## What to change in scripts

If a script used `sm log --json` to read CI or mergeability, switch that part to `sm status`. Keep `sm log --json` for branch graph, parent, PR number/state, current branch, and needs-restack data.

If a script used `sm doctor` or `sm status` interchangeably, use the explicit command:

- `sm doctor` for local metadata health.
- `sm status` for GitHub PR health.

## Why this is breaking

The default meaning of `sm log` changes from "inspect the stack and possibly refresh GitHub state" to "show the local stack immediately." That makes common navigation faster and more predictable, but callers that relied on live CI/mergeability from `sm log` must move to `sm status`.
