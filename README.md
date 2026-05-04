# stac-man

`sm` — a CLI for stacked pull requests. Free, local-only, no IDE plugin, no SaaS, no login. Stack metadata lives in your local git config; pull-request operations are delegated to the [GitHub CLI (`gh`)](https://cli.github.com/).

📚 **Documentation: [bluegardenproject.github.io/stac-man](https://bluegardenproject.github.io/stac-man/)**

## Install

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.sh | bash
```

**Windows (PowerShell)**

```powershell
iwr -useb https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.ps1 | iex
```

Verify with `sm --version`. Full install notes: [Get Started → Install](https://bluegardenproject.github.io/stac-man/get-started/install).

## Quickstart

Start a feature stack from trunk:

```bash
git switch main
sm create feat/auth-models
```

Edit your files, then commit (auto-restacks descendants):

```bash
sm modify -a -m "models: add User"
```

Stack a second branch on top of the first, edit, commit:

```bash
sm create feat/auth-handlers
```

```bash
sm modify -a -m "handlers: /login"
```

Inspect the tree, then push the whole stack as PRs with bases wired:

```bash
sm log
```

```bash
sm submit --stack
```

That's the whole loop. Walk through it end-to-end at [Your first stack](https://bluegardenproject.github.io/stac-man/get-started/first-stack).

## Interactive cockpit

Run `sm` with no arguments in a terminal and you land in the **cockpit** — a Bubble Tea TUI over the same service layer the CLI uses. One screen for the whole stack, single-key actions for every common verb (`enter` checkout, `r` restack, `m` modify, `s` submit, `d` diff, `?` help, `ctrl+p` command palette), an in-TUI conflict resolver for paused rebases, and a per-commit diff viewer. Pipes, CI, and any other non-TTY context still get `sm --help`, so existing scripts are unaffected. Full tour: [Concepts → Cockpit](https://bluegardenproject.github.io/stac-man/concepts/cockpit).

## Documentation

- [**Get Started**](https://bluegardenproject.github.io/stac-man/get-started/install) — install, first stack in 5 minutes.
- [**Concepts**](https://bluegardenproject.github.io/stac-man/concepts/) — stacks, trunk, restack, sync, parent metadata.
- [**Commands**](https://bluegardenproject.github.io/stac-man/commands/) — every subcommand and flag.
- [**Recipes**](https://bluegardenproject.github.io/stac-man/recipes/) — split a fat branch, land the bottom, recover from a bad rebase.
- [**Troubleshooting**](https://bluegardenproject.github.io/stac-man/troubleshooting) and [**FAQ**](https://bluegardenproject.github.io/stac-man/faq).

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

Commits must follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) and Go files must pass `gofmt`. Both rules are enforced locally by the hooks in [`.githooks/`](.githooks/) (wired up by `make setup`) and re-checked in CI.

Releases are driven by [Release Please](https://github.com/googleapis/release-please) on `main`. See the [release-please workflow](.github/workflows/release-please.yml) for the cross-compile matrix.

## License

TBD.
