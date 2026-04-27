# stac-man

`sm` — a CLI for stacked pull requests, in the spirit of Graphite's `gt`.

Documentation is being written. See `cmd/` for the command surface and `internal/` for the architecture once filled in.

## Setup

After cloning, run once:

```bash
make setup
```

This wires up the in-repo git hooks (via `core.hooksPath`) and makes them executable. Common targets like `make build` and `make test` re-run `setup` automatically, so the hook stays active even if you skip it on day one.

### Commit message policy

Commits must follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/), e.g. `feat(git): add typed accessors`. Enforcement runs in two places:

- **Locally**: a `commit-msg` hook (see [`.githooks/commit-msg`](.githooks/commit-msg)) blocks invalid messages before the commit lands.
- **In CI**: a GitHub Actions workflow re-runs the same check on every PR. This is the gate — local hooks can be skipped with `--no-verify`, CI cannot.

Both run the same validator: [`scripts/check-commit-msg.sh`](scripts/check-commit-msg.sh).

If you really need to bypass the local hook for an exceptional case:

```bash
git commit --no-verify
```

The CI check will still run on the PR.
