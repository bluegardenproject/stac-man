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

1. Calls the GitHub releases API for `philipptpunkt/stac-man` to find the latest tag.
2. Compares against the embedded `Version` of the running binary.
3. If newer, runs the platform-appropriate install script (curl on Linux/macOS, iwr on Windows). Otherwise prints "You're already on the latest version."

## Dev builds

Builds with `Version == "dev"` (from `make build` rather than a tagged release) skip the network check and print a hint to install a release first:

```
dev build — install a release to enable self-updates
  see the README for the curl install one-liner.
```

## When this can't help

- If `sm` itself is broken and won't run, fall back to the install one-liner from [Get Started → Install](/get-started/install).
- If your `~/.stac-man/` directory has been moved, `sm update` will install into the default location, not the new one.

## See also

- [`sm version`](./version) — what `sm update` reads to compare against.
- [Get Started → Install](/get-started/install) — manual install path.
