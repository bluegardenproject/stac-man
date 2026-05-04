---
description: Audit stac-man docs for drift against the binary surface, the contributor skill, and the user-facing skill. Use before each release-please cut and on demand whenever a new command, flag, or workflow lands. Produces a structured report with file paths and proposed edits; the agent then fixes or opens issues.
---

# Documentation drift audit

Docs go stale the moment a flag is added or a command is renamed. A build-time autogen step would catch the cobra surface but miss the substance — workflow recipes, conceptual explanations, branch-tree examples. This skill walks you through both surfaces in one pass.

## When to run

- Before every release-please cut (the workflow that opens release PRs).
- On demand when a new `cmd/` file lands or a new flag is added.
- When the user reports docs that don't match the binary.

Skill is the source of truth for *what* to check. Deliberately not a CI gate — false positives shouldn't block a release.

## What you'll need

- The repo at `~/.../stac-man/` (or wherever it's cloned).
- A working Go toolchain (`go run` for `tools/docscan`).
- Read access to:
  - `cmd/*.go` (cobra source)
  - `docs/web/commands/*.md` (one page per command)
  - `docs/web/recipes/*.md` (multi-step workflows)
  - `docs/web/concepts/*.md` (mental model)
  - `docs/skills/stac-man/SKILL.md` (end-user agent skill, mirrored into other repos)
  - `.cursor/skills/stac-man-dev/SKILL.md` (contributor skill)

## The five checks

Work through them in order. Each yields a section in the final report.

### 1. Every cobra command has a page

Source of truth: `go run ./tools/docscan` — emits JSON of the cobra tree. Parse with `jq` and check.

```bash
go run ./tools/docscan > /tmp/sm-cobra.json
jq -r '.commands[] | select(.path != "sm") | .path' /tmp/sm-cobra.json | sort
ls docs/web/commands/*.md | xargs -L1 basename | sed 's/\.md$//' | sort
```

Compare:

- A command in the cobra output without a corresponding `docs/web/commands/<name>.md` is **missing documentation**.
- A `docs/web/commands/*.md` page without a matching cobra command is **stale documentation**.

Two known mappings where one page covers multiple commands:

| Page | Covers cobra commands |
|---|---|
| `docs/web/commands/nav.md` | `up`, `down`, `top`, `bottom` |
| `docs/web/commands/continue.md` | `continue`, `abort` |

Treat these as covered; don't flag them as missing.

### 2. Every documented flag still exists

For every command page, list flags mentioned in the page (rows of the "Flags" table) and compare to the JSON surface:

```bash
jq -r '.commands[] | select(.name == "modify") | .flags[].name' /tmp/sm-cobra.json
```

- A flag in a doc page that's not in the cobra output is **stale**.
- A flag in the cobra output that's not mentioned in the page (and isn't a global like `--no-color` / `--verbose` already documented elsewhere) is **undocumented**.

Page format reminder: each command page has a `## Flags` section with a markdown table (`| Flag | Description |`). The drift check parses the first cell of each row.

### 3. Recipes match current `sm` output shape

Recipes use rendered tree fragments like:

```
* feat/web-form
  └─ feat/api-endpoints
     └─ feat/db-schema
        └─ main
```

Spot-check by:

1. Setting up a small temporary repo (`mktemp -d`, `git init`, two stacked branches).
2. Running `sm log` against it.
3. Confirming the format on the page matches.

If `sm log`'s rendering changed (markers, indentation, PR pill format), the recipes need updating. Flag every recipe page that shows a `sm log` snippet.

Recipe pages currently in scope:

- `docs/web/recipes/split-fat-branch.md`
- `docs/web/recipes/land-bottom-of-stack.md`
- `docs/web/recipes/recover-from-bad-rebase.md`
- `docs/web/recipes/review-someones-stack.md`
- `docs/web/recipes/absorb-fixups.md`

### 4. Skills mirror the docs

Three skill files describe `sm` from different angles. Drift between them is a substance bug.

- `docs/skills/stac-man/SKILL.md` — what end-user agents drive `sm` to do.
- `.cursor/skills/stac-man-dev/SKILL.md` — how contributors work on this repo.
- `docs/web/` — public-facing prose.

Cross-check:

- Every "decision rule" (e.g. "user says 'rebase' → use `sm restack`") in the end-user skill should be supported by a real command in the docs site. If a rule references a command that's been removed, flag it.
- Every "NEVER" in either skill should appear as a "what NOT to do" callout somewhere in the relevant docs page (typically the [Recover from a bad rebase](docs/web/recipes/recover-from-bad-rebase.md) recipe or a command page).
- Workflow examples in the skills (the canonical workflow block) should match the [first-stack tutorial](docs/web/get-started/first-stack.md) verbatim or close to it.

### 5. New commands have a recipe touchpoint

For commands shipped since the last release-please tag:

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD -- cmd/
```

Every new file under `cmd/` should be referenced from at least one recipe page. A command page on its own is necessary but not sufficient — recipes are where users learn *when* to reach for the command, not just what flags it has.

If a new command has no recipe mention, suggest where it would fit (e.g. a new recipe, or a "see also" addition to an existing one).

## The report

Output a markdown report shaped like this. The user (or you) will fix issues from it.

```markdown
# stac-man docs drift report — <date>

## 1. Missing command pages
- [ ] `sm <name>` — no `docs/web/commands/<name>.md`. Suggested: copy the closest existing page as a template.

## 2. Stale flags
- [ ] `docs/web/commands/<page>.md` documents `--<flag>` which no longer exists on `sm <command>`.

## 3. Undocumented flags
- [ ] `sm <command>` has flag `--<flag>` not mentioned in `docs/web/commands/<page>.md`.

## 4. Recipe drift
- [ ] `docs/web/recipes/<page>.md` shows a `sm log` fragment that doesn't match current output.

## 5. Skill drift
- [ ] `docs/skills/stac-man/SKILL.md` references `sm <gone>` which has been removed.

## 6. New commands without recipe touchpoints
- [ ] `sm <new>` (added in <commit>) has no recipe mention. Suggested home: `docs/web/recipes/<page>.md`.
```

For every item, include:

- The exact file path.
- A line range or section heading where the fix should land.
- A one-sentence proposed edit.

## Doing the fixes

After producing the report:

1. Fix items category-by-category (missing pages first — they unblock everything else).
2. Re-run the relevant checks after each fix to confirm.
3. Don't bundle unrelated content rewrites into this audit — keep the diff focused. If something is poorly written but accurate, file an issue and move on.

## Future-proofing

When this skill stabilizes, the highest-signal checks (#1 "every command has a page", #2 "stale flags") are the candidates to lift into a non-blocking CI lint. The skill stays the canonical place; CI is just the early-warning layer.
