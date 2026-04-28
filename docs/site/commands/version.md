# sm version

Print version, build, and platform info.

## Synopsis

```
sm version
sm --version    # short form: just the version string
```

## Example output

```
stac-man v2.0.0
  Built:    2026-04-15T11:32:01Z
  Go:       go1.25
  Platform: darwin/arm64
  Config:   /Users/me/.config/stac-man/config.yaml
```

## What's reported

| Line | Source |
|---|---|
| Version | Embedded at release time via `-X main.Version=…` ldflags. Reads `dev` for local builds. |
| Built | Embedded `BuildTime` (ISO-8601 UTC). |
| Go | `runtime.Version()` of the binary. |
| Platform | `GOOS/GOARCH` of the binary. |
| Config | Resolved path of `~/.config/stac-man/config.yaml` (whether or not the file exists). |

## See also

- [`sm update`](./update) — install the latest release.
- [Get Started → Install](/get-started/install) — how the binary is delivered.
