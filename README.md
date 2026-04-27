# stac-man

`sm` — a CLI for stacked pull requests, in the spirit of Graphite's `gt`. Free, local-only, no IDE plugin. Stack metadata lives in your local git config; pull-request operations are delegated to the [GitHub CLI (`gh`)](https://cli.github.com/).

## Why

If you've worked with Graphite, the workflow is the same: small, dependent branches that each become their own PR, with each PR's base set to the branch below it. `sm` keeps that workflow without a SaaS or login.

## Install

Build from source (Go 1.25+):

```bash
git clone https://github.com/philipptpunkt/stac-man.git
cd stac-man
make build
sudo install -m 0755 sm /usr/local/bin/sm
```

Verify:

```bash
sm --version
gh auth status   # only needed for `sm submit`
```

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

## Command surface (v1)

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

## Roadmap

- **v1** (current) — the command surface above; CLI only.
- **v2** — interactive TUI menu for the most common flows (`sm`, no subcommand).
- **v3** — release process (signed binaries, Homebrew tap), full docs website.

## License

TBD.
