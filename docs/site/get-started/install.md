# Install

`sm` is a single static Go binary. There is no service to sign up for, no project to create.

## One-liner — Linux & macOS

```bash
curl -fsSL https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.sh | bash
```

## One-liner — Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/philipptpunkt/stac-man/main/scripts/install.ps1 | iex
```

The installer downloads the latest release binary into `~/.stac-man/sm` (or `%USERPROFILE%\.stac-man\sm.exe` on Windows) and adds that directory to your shell's `PATH`. You may need to restart your shell or `source` your shell rc file the first time.

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

```bash
sm update           # install the latest release in-place
sm update --check   # only report whether a newer release exists
```

`sm update` shells out to the same install one-liner above. Dev builds (`Version == "dev"`) skip the network check and print a hint instead.

## Build from source

For contributors or platforms without a published binary (Go 1.25+):

```bash
git clone https://github.com/philipptpunkt/stac-man.git
cd stac-man
make build         # → ./sm with embedded version + build time
```

## Shell completions

Tab-completes subcommands and tracked branch names on `sm checkout`, `sm parent`, `sm move`, `sm show`.

```bash
# zsh
sm completion zsh > "${fpath[1]}/_sm"        # restart your shell

# bash
sm completion bash > /etc/bash_completion.d/sm

# fish
sm completion fish > ~/.config/fish/completions/sm.fish

# powershell
sm completion powershell > sm.ps1; . ./sm.ps1
```

Now jump to [your first stack](./first-stack) for the canonical workflow.
