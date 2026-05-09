> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# Install

`sm` is a single static Go binary. There is no service to sign up for, no project to create.

## One-liner — Linux & macOS

```bash
curl -fsSL https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.sh | bash
```

## One-liner — Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/install.ps1 | iex
```

The installer downloads the latest release binary into `~/.stac-man/sm` (or `%USERPROFILE%\.stac-man\sm.exe` on Windows) and adds that directory to your shell's `PATH`. You may need to restart your shell or `source` your shell rc file the first time.

::: details Which file does the installer touch?
| Shell | File the installer writes to |
|---|---|
| zsh (incl. oh-my-zsh) | `${ZDOTDIR:-$HOME}/.zshrc` |
| bash on Linux | `~/.bashrc` |
| bash on macOS | `~/.bashrc`, plus `~/.bash_profile` if it already exists (login shells only read the latter) |
| fish | `~/.config/fish/conf.d/stac-man.fish` (uses `fish_add_path`) |
| anything else | `~/.profile` (with a warning to add `~/.stac-man` to `PATH` manually if your shell doesn't source it) |

Each line is tagged with a `# stac-man (auto-added by install.sh)` marker and is idempotent: re-running the installer will not produce duplicates.
:::

## Verify

```bash
sm --version       # short version string
sm version         # full styled output: build time, platform, config path
gh auth status     # only needed if you'll use `sm submit`
```

If `sm --version` errors, your `PATH` hasn't picked up `~/.stac-man/`. Open a new shell or add it manually:

```bash
export PATH="$HOME/.stac-man:$PATH"
```

## GitHub CLI

`sm submit`, `sm land`, and `sm get` shell out to [`gh`](https://cli.github.com/). Install it (`brew install gh`, `apt install gh`, etc.) and run `gh auth login` once. `sm` never stores GitHub tokens; `gh` handles that.

The non-PR commands (`create`, `modify`, `restack`, `sync`, `log`, `show`, `absorb`, `undo` …) work without `gh`.

## Updating

Install the latest release in place:

```bash
sm update
```

Or just check whether a newer release exists, without changing anything:

```bash
sm update --check
```

`sm update` shells out to the same install one-liner above. Dev builds (`Version == "dev"`) skip the network check and print a hint instead.

::: details How will I know there's an update?
`sm` checks the GitHub releases API in the background **at most once every 24 hours** and caches the latest tag in `~/.cache/stac-man/last-update-check.json`. When the running binary is older than the cached tag, every command prints a single dim line on `stderr` after it finishes:

```
↑ stac-man v0.3.0 is available (you're on v0.2.1). Run `sm update` to install.
```

The check is non-blocking — it never delays a command. The hint is suppressed when:

- stdout is not a TTY (so pipes and CI logs stay clean),
- the running binary is a dev build (no tagged release to compare against),
- the env var `NO_UPDATE_NOTIFIER` is set (standard opt-out, mirrors `npm`/`gh`),
- you're running `sm update`, `sm version`, `sm completion`, or `sm help` (those commands handle their own version output).
:::

## Uninstall

`sm` lives in three places: the binary at `~/.stac-man/`, optional user config at `~/.config/stac-man/`, and per-repo stack metadata inside each repo's `.git/config`. The uninstaller cleans up the first two and leaves the third alone — that metadata is harmless without `sm` and is picked up again on reinstall.

::: code-group

```bash [Linux / macOS]
curl -fsSL https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/uninstall.sh | bash
```

```bash [Linux / macOS — also remove config]
curl -fsSL https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/uninstall.sh | bash -s -- --purge
```

```powershell [Windows]
iwr -useb https://raw.githubusercontent.com/bluegardenproject/stac-man/main/scripts/uninstall.ps1 | iex
```

:::

The script:

1. Removes `~/.stac-man/` (binary).
2. Strips the `# stac-man (auto-added by install.sh)` marker line and the following `export PATH=...` from `~/.zshrc`, `~/.bashrc`, `~/.bash_profile`, and `~/.profile` if present. A hand-edited rc that doesn't have the marker is left alone, with a warning.
3. Deletes `~/.config/fish/conf.d/stac-man.fish` (fish-only).
4. Asks before removing `~/.config/stac-man/`. Pass `--purge` to skip the prompt, `--keep-config` to skip the step entirely. When piped from `curl` (no TTY) the default is to keep config.

To undo it manually instead:

```bash
rm -rf ~/.stac-man ~/.config/stac-man
# delete the line referencing ~/.stac-man from your shell rc
```

Per-repo metadata, if you want it gone, lives under `branch.*.stac-man-*` and `stac-man.trunk` in the local git config:

```bash
git config --local --get-regexp '^stac-man\.|^branch\.[^.]+\.stac-man-' | cut -d. -f1-2 | sort -u | \
    xargs -n1 git config --local --remove-section
```

## Build from source

For contributors or platforms without a published binary (Go 1.25+):

```bash
git clone https://github.com/bluegardenproject/stac-man.git
cd stac-man
make build         # → ./sm with embedded version + build time
```

## Shell completions

Tab-completes subcommands and tracked branch names on `sm checkout`, `sm parent`, `sm move`, `sm show`.

Pick the tab for your shell, paste, then restart your shell so the new completions load.

::: code-group

```zsh [zsh]
sm completion zsh > "${fpath[1]}/_sm"
```

```bash [bash]
sm completion bash > /etc/bash_completion.d/sm
```

```fish [fish]
sm completion fish > ~/.config/fish/completions/sm.fish
```

```powershell [powershell]
sm completion powershell > sm.ps1
. ./sm.ps1
```

:::

Now jump to [your first stack](./first-stack) for the canonical workflow.
