# stac-man

`sm` — a CLI for stacked pull requests, in the spirit of Graphite's `gt`. Free, local-only, no IDE plugin. Stack metadata lives in your local git config; pull-request operations are delegated to the [GitHub CLI (`gh`)](https://cli.github.com/).

## Why

If you've worked with Graphite, the workflow is the same: small, dependent branches that each become their own PR, with each PR's base set to the branch below it. `sm` keeps that workflow without a SaaS or login.

## Install

One-liner (Linux / macOS):

```bash
curl -fsSL https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.sh | bash
```

PowerShell (Windows):

```powershell
iwr -useb https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.ps1 | iex
```

This downloads the latest release binary into `~/.stac-man/sm` (or `%USERPROFILE%\.stac-man\sm.exe` on Windows) and adds that directory to your shell's `PATH`. You may need to restart your shell or `source` your shell rc file the first time.

Verify:

```bash
sm --version
sm version          # full styled output (build time, platform, config path)
gh auth status      # only needed for `sm submit`
```

### Build from source

For contributors or platforms without a published binary (Go 1.25+):

```bash
git clone https://github.com/philipptpunkt/stac-man.git
cd stac-man
make build          # → ./sm with embedded version + build time
```

## Updating

```bash
sm update           # install the latest release in-place
sm update --check   # only report whether a newer release exists
```

`sm update` shells out to the same install one-liner above. Dev builds (`Version == "dev"`) skip the network check and print a hint instead.

If `sm` itself is broken, fall back to the curl/iwr one-liner from the [Install](#install) section.

## Quickstart

```bash
git switch main
sm create feat/auth-models           # branches off main
$EDITOR …
sm modify -a -m "models: add User"   # commit + auto-restack descendants

sm create feat/auth-handlers         # stacks on feat/auth-models
$EDITOR …
sm modify -a -m "handlers: /login"

sm log                               # see the tree
sm submit --stack                    # push + open PRs with bases wired
```

## Command surface

### Core (v1)

| Command | What it does |
|---|---|
| `sm create <name>` | Branch off HEAD, parent = previous branch. `-m` / `-a` to commit at the same time. |
| `sm modify [-c] [-a] -m <msg>` | Amend (default) or commit, then restack descendants. |
| `sm log` / `sm ls` | Print the stack tree with PR + restack status. |
| `sm checkout [name]` / `sm co` | Switch HEAD; without arg, list tracked branches. |
| `sm up` / `sm down` / `sm top` / `sm bottom` | Walk the stack. |
| `sm restack` | Rebase chain onto current parent tips. Pauses on conflict. |
| `sm continue` / `sm abort` | Resume or bail out of a paused restack/sync. |
| `sm sync` | Pull trunk, delete merged branches, restack survivors. |
| `sm submit [--stack] [--draft]` | Push + open/update PRs via `gh`. |
| `sm track [--parent X]` / `sm untrack [--reparent]` | Adopt or forget existing branches. |
| `sm parent [--set X]` / `sm children` | Inspect or change relationships. |
| `sm fold [-m msg]` | Squash branch into parent. |
| `sm doctor` / `sm status` | Sanity-check metadata vs. git state. |

### v2 additions

| Command | What it does |
|---|---|
| `sm absorb [--base X]` | Auto-route uncommitted hunks into the right ancestor commits via [`git-absorb`](https://github.com/tummychow/git-absorb), then restack descendants. |
| `sm move --onto X [branch]` | Reparent a branch (and its subtree) onto a new base; descendants ride along. |
| `sm land [--squash\|--merge\|--rebase] [--force]` | Merge the bottom-most PR via `gh pr merge` and run `sm sync` to clean up. CI-green by default. |
| `sm split [--names a,b,c --commits 1-2,3,4-5]` | Decompose the current branch into a chain of smaller branches (one per commit by default). |
| `sm show [branch] [--json]` | Detailed branch view: parent, children, ahead/behind, PR, commit list. `--json` for scripts/agents. |
| `sm get <PR-number>` | Fetch a colleague's stack locally and reproduce its parent edges, then print `sm log`. |
| `sm undo [--dry-run]` | Reflog-style rollback of the most recent stac-man op (last 50 ops kept under `.git/stac-man/history.json`). |
| `sm completion <shell>` | Generate completion scripts for bash / zsh / fish / powershell. |

### Enable completions

Tab-completes subcommands and tracked branch names on `sm checkout`, `sm parent`, `sm move`, `sm show`.

```bash
# zsh
sm completion zsh > "${fpath[1]}/_sm"      # then restart your shell

# bash
sm completion bash > /etc/bash_completion.d/sm

# fish
sm completion fish > ~/.config/fish/completions/sm.fish

# powershell
sm completion powershell > sm.ps1; . ./sm.ps1
```

## AI agent integration

`stac-man` ships skill files so AI agents (Cursor, Claude Code, etc.) know when and how to drive `sm` instead of plain git. Two flavors:

- **End-user skill** — for *consuming* `sm` in your own repo: [`docs/skills/stac-man/SKILL.md`](docs/skills/stac-man/SKILL.md). Copy this into your project's agent skills folder (e.g. `.cursor/skills/stac-man/SKILL.md` or your Claude skills directory).
- **Contributor skill** — for *developing on* this repo: [`.cursor/skills/stac-man-dev/SKILL.md`](.cursor/skills/stac-man-dev/SKILL.md). Loaded automatically when working in this repository.

Quick install for a Cursor-using project:

```bash
mkdir -p <your-repo>/.cursor/skills/stac-man
curl -fsSL https://raw.githubusercontent.com/philipptpunkt/stac-man/main/docs/skills/stac-man/SKILL.md \
  -o <your-repo>/.cursor/skills/stac-man/SKILL.md
```

A full docs site is planned for v3 alongside a release process.

## How state is stored

- **Per-repo**, in `.git/config`:
  - `stac-man.trunk`, `stac-man.version`
  - `branch.<name>.stac-man-parent`, `branch.<name>.stac-man-parent-sha`, `branch.<name>.stac-man-pr`
- **Per-user**, optional, in `~/.config/stac-man/config.yaml`: color, default base branch, draft-PR default. The file is optional; `sm` ships defaults.
- **Resume state** during a paused rebase: `.git/stac-man/restack.json` (auto-managed).
- **Undo history**: `.git/stac-man/history.json` (newest-first, capped at 50 entries; consumed by `sm undo`).

`sm` never stores GitHub tokens — `gh` handles that.

## Development

```bash
make setup            # wires the in-repo git hooks (run once after clone)
make build            # → ./sm
make test             # go test ./...
make vet              # go vet ./...
```

### Commit message policy

Commits must follow [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/), e.g. `feat(git): add typed accessors`. Enforcement:

- **Local**: `commit-msg` hook in [`.githooks/commit-msg`](.githooks/commit-msg).
- **CI**: GitHub Actions workflow runs the same validator.

Both call [`scripts/check-commit-msg.sh`](scripts/check-commit-msg.sh).

## Releasing

Releases are driven by [Release Please](https://github.com/googleapis/release-please) using the Conventional Commits in `main`:

1. Push a `feat:` / `fix:` / `perf:` / etc. commit to `main`.
2. The [Release Please workflow](.github/workflows/release-please.yml) opens (or updates) a release PR that bumps the version in [`main.go`](main.go), updates [`CHANGELOG.md`](CHANGELOG.md), and bumps [`.release-please-manifest.json`](.release-please-manifest.json).
3. Merging that PR creates a `vX.Y.Z` git tag plus a GitHub Release.
4. The same workflow then cross-compiles and uploads five binaries to the release: `sm-linux-amd64`, `sm-linux-arm64`, `sm-darwin-amd64`, `sm-darwin-arm64`, `sm-windows-amd64.exe`.
5. The install script picks up the new asset on its next run, and `sm update` will see the new tag.

Conventional Commit types are mapped to changelog sections in [`release-please-config.json`](release-please-config.json) (Features / Bug Fixes / Performance / Reverts / Documentation / Misc).

### Repo setup (one-time)

The release workflow needs a `PAT_TOKEN` secret with `repo` + `workflow` scope so that the release-please action can open PRs and trigger the asset-upload step. Add it under **Settings → Secrets and variables → Actions**.

You can also reproduce the cross-compile locally without publishing:

```bash
make build-all      # → dist/sm-{linux,darwin,windows}-{amd64,arm64}{.exe}
make release        # clean + build-all
```

## Roadmap

- **v1** — the v1 command surface above; CLI only.
- **v2.0** (current) — Graphite parity additions: `absorb`, `move`, `land`, `split`, `show`, `get`, `undo`, shell completions.
- **v2.1** — interactive TUI menu for the most common flows (`sm`, no subcommand).
- **v3** — signed binaries, Homebrew tap, full docs website.

## License

TBD.
