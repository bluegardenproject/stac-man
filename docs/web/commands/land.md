# sm land

Merge the bottom-most PR in your stack and run `sm sync` to clean up locally. CI-green by default.

## Synopsis

```
sm land [--squash | --merge | --rebase] [--force]
```

## Flags

| Flag | Description |
|---|---|
| `--squash` | Merge with the squash strategy (default). |
| `--merge` | Merge with a merge commit. |
| `--rebase` | Merge with a rebase strategy. |
| `--force` | Skip the CI-green gate. Use with care. |

The three strategy flags are mutually exclusive. Use whichever your repo's branch protection allows.

## Examples

Merge the bottom-most PR with squash, then sync:

```bash
sm checkout feat/web-form    # any branch in the stack
sm land
```

Force-merge a red CI:

```bash
sm land --force
```

Rebase-merge instead of squash:

```bash
sm land --rebase
```

## What it does

1. Walks from current branch toward trunk and finds the lowest tracked branch with a recorded PR.
2. Verifies CI is green via `gh pr checks` (skipped with `--force`).
3. Calls `gh pr merge` with the chosen strategy.
4. Runs [`sm sync`](./sync) — fetches origin, fast-forwards trunk, deletes the merged branch locally, re-parents children onto trunk, retargets their PR bases on GitHub.

## What it does NOT do

- Land does not merge multiple PRs in one shot. Run it again to land the next bottom.
- Land does not push. Sync handles the local cleanup; the merge happens through `gh`.

## See also

- [`sm sync`](./sync) — the cleanup pass that runs implicitly.
- [`sm submit`](./submit) — open / update PRs in the first place.
- [Recipes → Land the bottom of a stack](/recipes/land-bottom-of-stack).
