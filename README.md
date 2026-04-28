# stac-man

`sm` — a CLI for stacked pull requests. Free, local-only, no IDE plugin, no SaaS, no login. Stack metadata lives in your local git config; pull-request operations are delegated to the [GitHub CLI (`gh`)](https://cli.github.com/).

📚 **Documentation: [philipptpunkt.github.io/stac-man](https://philipptpunkt.github.io/stac-man/)**

## Install

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.sh | bash

# Windows (PowerShell)
iwr -useb https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.ps1 | iex
```

Verify with `sm --version`. Full install notes: [Get Started → Install](https://philipptpunkt.github.io/stac-man/get-started/install).

## Quickstart

```bash
git switch main
sm create feat/auth-models           # branches off main, parent=main
$EDITOR …
sm modify -a -m "models: add User"   # commit + auto-restack descendants

sm create feat/auth-handlers         # stacks on feat/auth-models
$EDITOR …
sm modify -a -m "handlers: /login"

sm log                               # see the tree
sm submit --stack                    # push + open PRs with bases wired
```

That's the whole loop. Walk through it end-to-end at [Your first stack](https://philipptpunkt.github.io/stac-man/get-started/first-stack).

## Documentation

- [**Get Started**](https://philipptpunkt.github.io/stac-man/get-started/install) — install, first stack in 5 minutes.
- [**Concepts**](https://philipptpunkt.github.io/stac-man/concepts/) — stacks, trunk, restack, sync, parent metadata.
- [**Commands**](https://philipptpunkt.github.io/stac-man/commands/) — every subcommand and flag.
- [**Recipes**](https://philipptpunkt.github.io/stac-man/recipes/) — split a fat branch, land the bottom, recover from a bad rebase.
- [**Troubleshooting**](https://philipptpunkt.github.io/stac-man/troubleshooting) and [**FAQ**](https://philipptpunkt.github.io/stac-man/faq).

## AI-agent integration

`sm` ships skill files so AI agents (Cursor, Claude Code, etc.) know when to drive `sm` instead of plain `git`:

- **End-user skill** — drop-in for any repo: [`docs/skills/stac-man/SKILL.md`](docs/skills/stac-man/SKILL.md).
- **Contributor skill** — auto-loaded when working on this repo: [`.cursor/skills/stac-man-dev/SKILL.md`](.cursor/skills/stac-man-dev/SKILL.md).

## Development

```bash
make setup            # wire up in-repo git hooks (run once after clone)
make build            # → ./sm
make test             # go test ./...
```

Commits must follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/). The `commit-msg` hook in [`.githooks/`](.githooks/) enforces this locally; CI does the same.

Releases are driven by [Release Please](https://github.com/googleapis/release-please) on `main`. See the [release-please workflow](.github/workflows/release-please.yml) for the cross-compile matrix.

## License

TBD.
