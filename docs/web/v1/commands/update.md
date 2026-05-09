> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

# sm update

Install the latest released version of `sm` in-place. Shells out to the same install one-liner used for first-time installation.

## Synopsis

```
sm update [--check]
```

## Flags

| Flag | Description |
|---|---|
| `--check` | Only report whether an update is available; do not install. |

## Examples

Check without installing:

```bash
sm update --check
# Current: v1.4.0
# Latest:  v2.0.0
# A newer version is available: v2.0.0
#   run `sm update` to install it.
```

Install:

```bash
sm update
```

## What it does

1. Calls the GitHub releases API for `bluegardenproject/stac-man` to find the latest tag.
2. Compares against the embedded `Version` of the running binary.
3. If newer, runs the platform-appropriate install script (curl on Linux/macOS, iwr on Windows). Otherwise prints "You're already on the latest version."

## Background update notification

You don't have to remember to run `sm update --check`. `sm` queries the GitHub releases API in the background at most once every 24 hours and caches the result in `~/.cache/stac-man/last-update-check.json` (or `$XDG_CACHE_HOME/stac-man/...` if set). When the running binary is older than the cached tag, every command prints a single dim line on `stderr` after it finishes:

```
↑ stac-man v0.3.0 is available (you're on v0.2.1). Run `sm update` to install.
```

The check is non-blocking. To opt out entirely, set `NO_UPDATE_NOTIFIER=1` in your shell. The notification is automatically suppressed for `sm update`, `sm version`, `sm completion`, `sm help`, dev builds, and non-TTY stdout.

## Dev builds

Builds with `Version == "dev"` (from `make build` rather than a tagged release) skip the network check and print a hint to install a release first:

```
dev build — install a release to enable self-updates
  see the README for the curl install one-liner.
```

## When this can't help

- If `sm` itself is broken and won't run, fall back to the install one-liner from [Get Started → Install](/v1/get-started/install).
- If your `~/.stac-man/` directory has been moved, `sm update` will install into the default location, not the new one.

## See also

- [`sm version`](./version) — what `sm update` reads to compare against.
- [Get Started → Install](/v1/get-started/install) — manual install path.
