# FAQ

## How is this different from Graphite / Aviator / Sapling / git-branchless?

`sm` is deliberately small.

| | `sm` | Graphite / Aviator | Sapling | git-branchless |
|---|---|---|---|---|
| SaaS / login | None | Required | Required (Meta) | None |
| IDE plugin | No | Optional | Optional | No |
| Storage | Git config | Cloud | Cloud + local | Local DB |
| GitHub PR ops | Through `gh` | Native | Native | Through `gh` |
| Cost | Free | Paid tiers | Free | Free |

If you want a hosted product with a web dashboard, Graphite or Aviator are great. If you want a local CLI that records parents in `.git/config` and shells out to `gh`, that's `sm`.

## Why no SaaS

Two reasons:

1. **Stacks aren't sensitive metadata.** They're three keys per branch in git config. There's nothing to host.
2. **Stacked PRs shouldn't require buy-in from the rest of your team.** With `sm`, you can stack on a repo where nobody else does. Reviewers see normal PRs with normal bases.

## Does `sm` work without `gh`?

Mostly yes. The GitHub-touching commands (`submit`, `land`, `get`, the PR-status part of `log`) need `gh`. Everything else works offline:

- `create`, `modify`, `restack`, `sync` (without retargeting), `log --no-pr`, `show`, `parent`, `children`, `track`, `untrack`, `move`, `split`, `fold`, `absorb`, `undo`, `doctor`, `checkout`, `up`/`down`/`top`/`bottom`, `version`, `update`.

So you can build and rearrange stacks fully offline, and only need `gh` when you're ready to push them as PRs.

## Where does the stack metadata live?

In `.git/config`, per branch:

```ini
[branch "feat/handlers"]
    stac-man-parent = feat/models
    stac-man-parent-sha = 4f1e2…
    stac-man-pr = 42
```

Plus `stac-man.trunk` and `stac-man.version` repo-level keys. Two transient JSON files appear under `.git/stac-man/` for paused restacks and the undo log; both are auto-managed.

See [Concepts → Parent metadata](/concepts/parent-metadata) for details.

## Why is metadata not pushed to origin?

Because:

1. **No SaaS, no shared store.** That's the project's whole point.
2. **The PR base graph on GitHub already encodes the same shape.** [`sm get <PR>`](/commands/get) reconstructs local metadata from a PR chain.

So when you switch machines, run `sm get <PR-of-the-top>` once and you're back in business.

## Can two people on the same team use `sm` independently?

Yes. The metadata is per-clone, so your tracking has no effect on your teammate's clone. As long as you're both opening PRs through `gh` (whether from `sm submit` or by hand), the PRs themselves are identical to anyone else's.

## What if my team's trunk isn't `main` or `master`?

```bash
git config --local stac-man.trunk develop
```

That's the only setting you'll ever need to change manually.

## Does `sm` ever force-push?

`sm submit` uses `--force-with-lease`. That refuses to push if origin has commits you haven't seen — preventing the classic "I clobbered my colleague's work" failure mode. `sm` never uses `git push --force` (the unsafe form).

## Can I rename `sm`?

Sure — the binary is just `sm`. Symlink or alias it to whatever you want:

```bash
ln -s ~/.stac-man/sm ~/.local/bin/stk
```

The rest of the docs assume `sm` because that's what everything looks like in help text.

## Will there be a TUI?

[Yes — planned for v2.1.](https://github.com/philipptpunkt/stac-man/blob/main/ROADMAP.md) `sm` (no args) will open a Bubble Tea menu over the current stack with single-letter shortcuts for every action. Today the picker is only used by `sm checkout` (no args).

## Will there be a web UI?

No. Out of scope for a CLI tool. If you want one, GitHub itself shows the stack view fine once `sm submit --stack` has wired the bases up.

## Is `sm`'s license compatible with my project?

License is currently TBD. We'll pin it before the v2.0 cut. (Issue tracker reflects this.)

## How do I report a bug?

[github.com/philipptpunkt/stac-man/issues](https://github.com/philipptpunkt/stac-man/issues). Please include the output of:

```bash
sm version
sm doctor
git config --list --local | grep stac-man
```

## How do I contribute

See `.cursor/skills/stac-man-dev/SKILL.md` in the repo for the contributor conventions (package layout, testing rules, Conventional Commits enforcement). PRs welcome.
