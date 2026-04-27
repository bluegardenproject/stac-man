---
description: Conventions for developing on the stac-man (sm) repo itself — package layout, where each command lives, how to add new ones, testing rules, and Conventional Commits enforcement.
---

# stac-man — internal development guide

This skill applies when an AI agent is writing code **inside** the `stac-man` repository (NOT when consuming the `sm` CLI from another project — for that, see the user-facing skill at `docs/skills/stac-man/SKILL.md`).

## Repository layout

```
stac-man/
  main.go                       # entrypoint: signal context → cmd.Execute
  cmd/                          # one cobra subcommand per file
    root.go                     # root command + global flags + register()
    create.go, log.go, …        # each calls register(cmd) from init()
  internal/
    git/                        # typed wrapper over the `git` binary
    gh/                         # typed wrapper over the `gh` binary
    store/                      # Store interface + gitconfig + memory impls
    stack/                      # Graph type, traversal, validation
    service/                    # orchestration: every user op is a method here
    restack/                    # rebase walk + on-disk resume state
    config/                     # optional ~/.config/stac-man/config.yaml
    history/                    # bounded undo log under .git/stac-man/history.json
    ui/, ui/theme/              # neon-synthwave styling (lipgloss)
```

## Layering rules

- **`cmd/` is thin.** A subcommand parses flags, builds a `service.Service` via `newService()`, and calls a single method. No domain logic in cobra files.
- **`internal/service/` owns orchestration.** Every user-facing operation is a method on `*Service`. A future TUI (v2) calls these directly without touching cmd/.
- **`internal/git/` and `internal/gh/`** are the only packages allowed to shell out. Other packages talk to git/gh via these typed clients.
- **`internal/store/`** is the metadata source of truth (`gitconfig` impl in production, `memory` impl in tests). Never read git config directly elsewhere.
- **`internal/stack/`** is pure: takes a `store.Store`, returns an in-memory `Graph`. No I/O after `Load`.
- **`internal/restack/`** is the rebase engine — walks a topo-ordered queue and persists resume state to `.git/stac-man/restack.json`.
- **`internal/history/`** persists the undo log. Every mutating service method MUST call `s.recordHistory(ctx, op, notes, branches)` before mutating, listing every branch the op may touch. `sm undo` rewinds the most recent entry.

## Adding a new subcommand

1. Add a file `cmd/<name>.go`. In `init()` build the cobra.Command and call `register(cmd)`.
2. Inside the cobra `RunE`, call `newService().<Method>(c.Context(), …)`.
3. Add the method in `internal/service/<area>.go`. Keep everything testable: do all I/O through the `*git.Client`, `*gh.Client`, and `store.Store` already on `*Service`.
4. If the method needs a new git operation, add a typed wrapper to `internal/git/client.go`. Don't shell out from service code directly.
5. Test the service method against `memory.Store` plus a fake `git` runner — never against the real binary.
6. Mutating methods record an undo entry: call `s.recordHistory(ctx, "<op>", "<notes>", []string{<every branch this op may touch>})` BEFORE the first mutation. The list should include the current branch, any branches whose tip or parent metadata may move, and (for ops that delete or create branches) the branches being deleted/created. Snapshots are best-effort — don't fail the op if history write fails.

## Testing rules

- **Unit-test the service layer** with `memory.Store` and fake `Runner` implementations (`internal/git/runner_test.go` shows the pattern).
- **Don't shell out to real `git` or `gh` from tests.** Integration tests that need a real git repo go in `tests/integration/` (temporary repos created with `t.TempDir()`).
- Run `go test ./... && go vet ./...` before every commit.
- Use `ReadLints` after substantive edits.

## Commit messages — Conventional Commits 1.0.0

The `commit-msg` hook in `.githooks/` enforces:

```
<type>(<scope>)!: <subject>
```

- `type` ∈ {build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test}
- `scope` is optional, lower-case (e.g. `feat(git): …`, `fix(restack): …`)
- The hook auto-skips merge / revert / fixup / squash / amend autosquash messages

Enable the hooks once per clone with:

```bash
git config core.hooksPath .githooks
```

## NEVER

- Add "made with Cursor", "Co-authored-by: Cursor", or any AI-tool attribution to commit messages or code comments. The `.cursor/rules/no-ai-attribution.mdc` rule covers this — read it.
- Add narration-style comments like `// increment the counter` — only document non-obvious intent.
- Bypass the hooks via `--no-verify` for normal work. Use it only for genuine emergencies.
- Reach across layers (cmd → store directly, service → git internals, etc.). Add a method to the right wrapper instead.

## Useful one-liners

```bash
# Build the binary into the repo root
go build -o sm .

# Run the full check
go test ./... && go vet ./...

# See what trunk stac-man auto-detected for this repo
git config --local --get stac-man.trunk
```
