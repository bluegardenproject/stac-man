> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm config

Inspect or edit the optional user config at `~/.config/stac-man/config.yaml` (or `$XDG_CONFIG_HOME/stac-man/config.yaml` if set).

The file is **optional**. `sm` works fine without it, with the documented defaults. Per-repo state — stack metadata, trunk, undo log — lives in each repo's own `.git/config` and `.git/stac-man/`, not in this file. Updating `sm` via `sm update` never touches your config.

## Synopsis

```
sm config path
sm config show
sm config edit
sm config init
```

## Subcommands

| Subcommand | What it does |
|---|---|
| `sm config path` | Print the absolute path that `sm` would read. |
| `sm config show` | Print the effective config (defaults + any overrides) as YAML. |
| `sm config edit` | Open the file in `$VISUAL` (or `$EDITOR`, falling back to `nano`/`vim`/`vi`); creates a defaults file first if missing. |
| `sm config init` | Write a default config file if one doesn't exist yet. No-op if it does. |

## Available keys

```yaml
color: auto                  # auto | always | never
default_base_branch: ""      # override per-repo trunk detection (empty = autodetect)
prefer_draft_prs: false      # if true, `sm submit` creates new PRs as drafts unless --draft=false
```

::: tip
Setting `color: never` is equivalent to running every command with `--no-color` or exporting `NO_COLOR=1`. The `--no-color` CLI flag still wins on a per-invocation basis.
:::

## Persistence across updates

The config file lives outside `~/.stac-man/` (which is what `sm update` overwrites), so it survives every update. To remove it, delete `~/.config/stac-man/` or run the [uninstaller](/v1/get-started/install#uninstall) with `--purge`.

## See also

- [Get Started → Install](/v1/get-started/install) — including the **Uninstall** section.
- [`sm submit`](./submit) — honors `prefer_draft_prs`.
- [`sm version`](./version) — also prints the resolved config path.
