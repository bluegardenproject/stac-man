---
layout: home

hero:
  name: stac-man
  text: Stacked PRs, locally.
  tagline: A small CLI for stacked pull requests. Free, local-only, no SaaS, no IDE plugin, no login.
  image:
    src: /stac-man-logo.png
    alt: stac-man
  actions:
    - theme: brand
      text: Get Started
      link: /v1/get-started/install
    - theme: alt
      text: Concepts
      link: /v1/concepts/
    - theme: alt
      text: View on GitHub
      link: https://github.com/bluegardenproject/stac-man

features:
  - title: Local-first metadata
    details: Stack relationships live in `.git/config`. No service to log into, no project to create. Drop the binary on your PATH and start stacking.
  - title: Stack-aware git verbs
    details: '`sm create`, `sm modify`, `sm restack`, `sm sync` — every mutating command rewrites descendants for you so the chain stays consistent.'
  - title: Interactive cockpit
    details: 'Bare `sm` on a TTY opens a Bubble Tea TUI: dashboard, conflict resolver, diff viewer, and a fuzzy command palette over every common verb.'
  - title: PRs through gh
    details: '`sm submit --stack` pushes every branch and opens or retargets PRs through the GitHub CLI. No tokens stored, no extra auth.'
  - title: Recover, undo, doctor
    details: Every mutation is journalled. `sm undo` rewinds the last op; `sm doctor` reports drift; `sm absorb` routes loose hunks back where they belong.
---

> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).


## Why another stacked-PR tool

Stacked pull requests — small, dependent branches that each become their own PR, with each PR's base set to the branch below it — make code review faster and keep changes reviewable. Every existing tool either ships as a SaaS, a paid IDE plugin, or a heavyweight rewrite of git. `sm` is a single Go binary that records the parent of each branch in git config and keeps the chain in sync as you iterate.

## In 30 seconds

Start a feature stack from trunk:

```bash
git switch main
sm create feat/auth-models
```

Edit your files, then commit (auto-restacks descendants):

```bash
sm modify -a -m "models: add User"
```

Stack a second branch on top of the first, edit, commit:

```bash
sm create feat/auth-handlers
```

```bash
sm modify -a -m "handlers: /login"
```

Inspect the tree, then push the whole stack as PRs with bases wired:

```bash
sm log
```

```bash
sm submit --stack
```

That's the whole loop. From here:

- [Install](/v1/get-started/install) the binary.
- Walk through your [first stack](/v1/get-started/first-stack) end to end.
- Skim the [concepts](/v1/concepts/) so the verbs make sense.
- Browse the [command reference](/v1/commands/).
