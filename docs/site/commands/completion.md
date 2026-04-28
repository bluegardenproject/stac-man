# sm completion

Generate the autocompletion script for `sm` in the named shell.

## Synopsis

```
sm completion <bash|zsh|fish|powershell>
```

## Examples

```bash
# zsh — install once, then restart your shell
sm completion zsh > "${fpath[1]}/_sm"

# bash
sm completion bash > /etc/bash_completion.d/sm

# fish
sm completion fish > ~/.config/fish/completions/sm.fish

# powershell
sm completion powershell > sm.ps1
. ./sm.ps1
```

## What it generates

A shell-specific script that wires:

- Subcommand completion (e.g. `sm cre<TAB>` → `create`).
- Tracked-branch-name completion on `sm checkout`, `sm parent`, `sm move`, `sm show`.
- Flag completion for every command (long flags, short flags, value enums where applicable).

## Verifying

After installing, type:

```bash
sm checkout <TAB>
```

The list of tracked branches plus trunk should appear.

## See also

- [Get Started → Install § Shell completions](/get-started/install#shell-completions) — quick install.
