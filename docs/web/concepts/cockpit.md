# Cockpit (interactive TUI)

Run `sm` with no arguments in a terminal and you land in the **cockpit** — a Bubble Tea TUI that puts the whole stack on one screen and turns every common verb into a single keystroke. It's the same surface as the CLI; `sm submit`, `sm restack`, `sm modify`, etc. all work exactly as they do from the shell. The cockpit just removes the "what was that flag again?" friction.

Bare `sm` opens the cockpit on a TTY:

```bash
sm
```

`sm --help` still prints help, even on a TTY:

```bash
sm --help
```

Every subcommand keeps its old behaviour — e.g. `sm log` always prints the tree:

```bash
sm log
```

In a pipe, CI, or any other non-TTY context, bare `sm` falls back to `sm --help` so existing scripts are unaffected.

## What's on screen

The cockpit has five screens; the dashboard is where you start.

### Dashboard
Two panes. The **left pane** is your stack tree, rendered from local metadata and cached GitHub status — current branch highlighted, needs-restack markers, PR pills, and the last known CI/mergeability badges when available. The **right pane** is the detail view of whatever row your cursor is on: parent, children, ahead/behind, PR state, and the recent commit list (powered by `sm show`).

On entry, the cockpit renders immediately from local data and the last cached PR/status snapshots. If tracked PRs exist, it starts a background GitHub status refresh and keeps the old badges visible while the request is in flight. When fresh status arrives, rows update in place; if the refresh fails, the dashboard stays visible and shows a non-blocking warning.

| Key | What it does |
|---|---|
| `↑` / `↓` (or `j` / `k`) | Move the cursor through the tree |
| `g` / `G` | Jump to the top / bottom |
| `enter` | Checkout the selected branch |
| `r` | Restack the selected branch |
| `t` / `T` | Track / untrack the selected branch |
| `m` | Amend the current commit (`sm modify`) |
| `f` | Fold the current branch into its parent |
| `a` | Run `sm absorb` |
| `u` | Undo the last `sm` op |
| `s` / `S` / `L` | Submit / sync / land (shells out so `gh` can prompt) |
| `d` | Open the diff viewer for the selected branch |
| `ctrl+r` (or `R`) | Reload the snapshot |
| `?` | Help overlay |
| `ctrl+p` (or `ctrl+k`) | Command palette |
| `q` | Quit |

Mutating actions on the trunk row are refused; the status line tells you why.

### Conflict resolver
If `sm restack`, `sm modify`, `sm sync`, or `sm submit` pause on a conflict, the cockpit auto-routes here. You see the rebase origin, the branch the conflict is on, the queue of branches still pending, and the unmerged file list pulled live from `git`.

| Key | What it does |
|---|---|
| `↑` / `↓` | Move through conflicting files |
| `e` | Open the selected file in `$EDITOR` |
| `c` | `sm continue` once you've staged the resolution |
| `A` | `sm abort` to bail out of the rebase |
| `esc` | Back to the dashboard |

When the rebase clears, the cockpit bounces back to the dashboard automatically.

### Diff viewer
Pressing `d` on a tracked branch opens a two-pane diff. **Left** lists the commits that branch adds on top of its parent (one row per `sm show` commit). **Right** is the rendered diff for the selected commit, fetched on demand via `git show` and cached per-SHA so stepping back and forth is instant.

| Key | What it does |
|---|---|
| `tab` / `n` | Next commit |
| `shift+tab` / `p` | Previous commit |
| `pgdn` / `ctrl+d` | Page down inside the diff |
| `pgup` / `ctrl+u` | Page up inside the diff |
| `esc` | Back to the dashboard |

### Command palette
`ctrl+p` opens a fuzzy-searchable list of every dashboard action (checkout, restack, submit, …), plus refresh actions such as `refresh GitHub status`. Type to filter, `enter` runs, `esc` closes. Useful when you forget a key binding or want to pick an action without leaving the keyboard.

### Help overlay
`?` brings up a categorised cheat sheet for every key the cockpit understands. The list is generated from the same keymap the action handlers use, so it can't drift.

## In-process vs. shell-out

To keep the UX coherent, the cockpit splits its work in two:

- **In-process (fast, local):** `checkout`, `restack`, `track`, `untrack`, `modify`, `fold`, `absorb`, `undo`, cached dashboard refreshes, GitHub status refreshes, plus the conflict resolver. These call the same `internal/service` layer the CLI uses, so paused-rebase semantics, the undo journal, cache updates, and `git config` updates behave identically.
- **Shell-out (`tea.ExecProcess`):** `submit`, `sync`, `land`. These can prompt through `gh` or take a long time, so the cockpit yields the terminal to the real `sm` subcommand and resumes when it exits. Output is exactly what you'd see running `sm submit` directly.

A network action that leaves the system in a paused rebase (e.g. `sm sync` hitting a conflict) auto-routes you into the conflict resolver on the next snapshot reload — no manual navigation needed.

## What the cockpit doesn't do

- It doesn't introduce new business logic. Anything you can do in the cockpit you can do from the CLI, and the underlying state changes are identical.
- It doesn't replace `sm log` / `sm show` for scripting. Use the CLI (with `--json` / `--porcelain`) when you want machine-readable output.
- It doesn't ship with extra dependencies. Bubble Tea and Lipgloss are already in the binary.
