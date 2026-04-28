---
layout: home

hero:
  name: stac-man
  text: Stacked PRs, locally.
  tagline: A small CLI for stacked pull requests. Free, local-only, no SaaS, no IDE plugin, no login.
  actions:
    - theme: brand
      text: Get Started
      link: /get-started/install
    - theme: alt
      text: Concepts
      link: /concepts/
    - theme: alt
      text: View on GitHub
      link: https://github.com/philipptpunkt/stac-man

features:
  - title: Local-first metadata
    details: Stack relationships live in `.git/config`. No service to log into, no project to create. Drop the binary on your PATH and start stacking.
  - title: Stack-aware git verbs
    details: '`sm create`, `sm modify`, `sm restack`, `sm sync` — every mutating command rewrites descendants for you so the chain stays consistent.'
  - title: PRs through gh
    details: '`sm submit --stack` pushes every branch and opens or retargets PRs through the GitHub CLI. No tokens stored, no extra auth.'
  - title: Recover, undo, doctor
    details: Every mutation is journalled. `sm undo` rewinds the last op; `sm doctor` reports drift; `sm absorb` routes loose hunks back where they belong.
---

## Why another stacked-PR tool

Stacked pull requests — small, dependent branches that each become their own PR, with each PR's base set to the branch below it — make code review faster and keep changes reviewable. Every existing tool either ships as a SaaS, a paid IDE plugin, or a heavyweight rewrite of git. `sm` is a single Go binary that records the parent of each branch in git config and keeps the chain in sync as you iterate.

## In 30 seconds

```bash
git switch main
sm create feat/auth-models           # branches off main
$EDITOR …
sm modify -a -m "models: add User"   # commit + auto-restack descendants

sm create feat/auth-handlers         # stacks on feat/auth-models
$EDITOR …
sm modify -a -m "handlers: /login"

sm log                               # see the tree
sm submit --stack                    # push + open PRs with bases wired
```

That's the whole loop. From here:

- [Install](/get-started/install) the binary.
- Walk through your [first stack](/get-started/first-stack) end to end.
- Skim the [concepts](/concepts/) so the verbs make sense.
- Browse the [command reference](/commands/).
