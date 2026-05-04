# Command reference

Every `sm` subcommand, grouped by intent. For the canonical workflow see [Your first stack](/get-started/first-stack); for multi-step recipes see [Recipes](/recipes/).

## Navigation

| Command | What it does |
|---|---|
| [`sm checkout`](./checkout) (alias: `co`) | Switch HEAD to a tracked branch (or pick interactively). |
| [`sm up` / `down` / `top` / `bottom`](./nav) | Walk the stack relative to current. |

## Mutation

| Command | What it does |
|---|---|
| [`sm create`](./create) | Branch off HEAD, record parent. `-m` / `-a` to commit at the same time. |
| [`sm modify`](./modify) | Amend (default) or commit, then restack descendants. |
| [`sm fold`](./fold) | Squash branch into parent. |
| [`sm split`](./split) | Decompose a branch into a chain of smaller ones. |
| [`sm move`](./move) | Reparent a branch (and its subtree) onto a new base. |

## Restack engine

| Command | What it does |
|---|---|
| [`sm restack`](./restack) (alias: `rs`) | Rebase chain onto current parent tips. Pauses on conflict. |
| [`sm continue` / `sm abort`](./continue) | Resume or bail out of a paused restack/sync. |
| [`sm sync`](./sync) | Pull trunk, delete merged branches, restack survivors. |

## PR ops

| Command | What it does |
|---|---|
| [`sm submit`](./submit) | Push + open/update PRs via `gh`. |
| [`sm land`](./land) | Merge the bottom-most PR and run `sm sync` to clean up. |
| [`sm get`](./get) | Fetch a colleague's stack locally and reproduce its parent edges. |

## Inspection

| Command | What it does |
|---|---|
| [`sm log`](./log) (alias: `ls`) | Print the stack tree with PR + restack status. |
| [`sm show`](./show) | Detailed branch view: parent, children, ahead/behind, PR, commits. |
| [`sm parent`](./parent) | Show or change the parent of a branch. |
| [`sm children`](./children) | List the direct children of a branch. |
| [`sm doctor`](./doctor) (alias: `status`) | Sanity-check stac-man metadata vs. git state. |

## Recovery

| Command | What it does |
|---|---|
| [`sm undo`](./undo) | Roll back the most recent stac-man operation. |
| [`sm absorb`](./absorb) | Auto-route uncommitted hunks into the right ancestor commits. |

## Setup & meta

| Command | What it does |
|---|---|
| [`sm track`](./track) | Adopt an existing branch into the stack graph. |
| [`sm untrack`](./untrack) | Remove a branch from the stack graph. |
| [`sm completion`](./completion) | Generate shell completion scripts. |
| [`sm config`](./config) | Inspect or edit the optional user config (`~/.config/stac-man/config.yaml`). |
| [`sm version`](./version) | Print version, build, and platform info. |
| [`sm update`](./update) | Install the latest released version. |

## Global flags

Every subcommand inherits these from the root:

| Flag | What it does |
|---|---|
| `--no-color` | Disable all color output (also respects the `NO_COLOR` env var). |
| `-v`, `--verbose` | Verbose output (prints underlying git/gh commands). |
