# tools/docscan

A small Go program that walks the `sm` cobra command tree and emits JSON.

It's the programmatic source for the [docs-drift-audit](../../.cursor/skills/docs-drift-audit/SKILL.md) Cursor skill — given a JSON snapshot of the binary surface, the skill diffs it against `docs/web/commands/*.md` to catch missing pages, stale flags, and undocumented features.

## Run

```bash
go run ./tools/docscan
go run ./tools/docscan | jq '.commands[] | {name, path, flags: [.flags[].name]}'
```

## Output schema (v1)

```json
{
  "binary": "sm",
  "genSchema": "v1",
  "commands": [
    {
      "name": "checkout",
      "use":  "checkout [branch]",
      "short": "Switch HEAD to a tracked branch (or trunk)",
      "long":  "...",
      "aliases": ["co"],
      "path": "sm checkout",
      "hasArgs": true,
      "flags": [
        { "name": "no-color", "usage": "...", "type": "bool", "default": "false" }
      ],
      "subcommands": []
    }
  ]
}
```

The shape is stable; `genSchema` will bump if the structure ever needs to break compatibly.

## Why a separate tool

cobra exposes a JSON dumper for completion data, but not for the surface a docs auditor cares about (flag descriptions, defaults, types, aliases, the canonical path). docscan is ~80 LOC of straightforward `cobra.Command` traversal — easier to keep stable than to wrangle with cobra's built-ins.
