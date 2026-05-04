# Review someone's stack

A teammate opened PR #142, and PR #142 is the top of a stack. You want to read the whole thing locally, bottom-up, with the parent edges intact.

## What you'd otherwise do

Without `sm`, you'd manually walk the PR chain — find each PR's base ref, check it out, repeat:

```text
gh pr view 142 --json baseRefName            # find the parent PR
gh pr checkout <parent-branch-name>
gh pr view <parent-pr-number> --json baseRefName
gh pr checkout <grandparent-branch-name>
# ...repeat down to trunk
```

Tedious, and you don't end up with parent metadata recorded — `sm log` won't render the tree.

## With sm

```bash
sm get 142
```

What this does, in order:

1. `gh pr view 142 --json baseRefName,headRefName` to find the chain.
2. Walks the `base_ref` chain down to trunk.
3. `gh pr checkout` on each branch.
4. Records parent metadata (`branch.<n>.stac-man-parent`, `-parent-sha`).
5. Records the PR number on each branch (`branch.<n>.stac-man-pr`).
6. Leaves HEAD on the top branch.
7. Prints `sm log`.

## Starting from PR #142, ending state

```
stac-man

main
└─ feat/db-schema  #140 merged
   └─ feat/api-endpoints  #141 open  CI ready
      └─ feat/web-form  ← current  #142 open  CI conflict
```

`feat/web-form` is the PR you want to review (HEAD is on it). `CI conflict` flags it as `CONFLICTING` on GitHub — typical when an ancestor has merged but the local chain hasn't been rebased onto trunk yet. Run `sm sync` to clean that up before reading the diff.

Now you can walk the chain bottom-up. Jump to the branch sitting directly on trunk (`feat/db-schema`):

```bash
sm bottom
```

Step up one branch (`feat/api-endpoints`):

```bash
sm up
```

And once more (`feat/web-form`):

```bash
sm up
```

Or jump directly:

```bash
sm checkout feat/api-endpoints
```

Each branch is real — you can run tests, set breakpoints, even commit fixups.

## Reading the stack diff

`sm show` on each branch tells you the commits unique to that PR:

```bash
sm checkout feat/api-endpoints
sm show
```

If you want a single diff scoped to one layer (no parent commits):

```bash
git log --reverse --patch feat/db-schema..feat/api-endpoints
```

## What `sm get` does NOT do

- It does not pull unsubmitted descendants. If the author has more branches stacked on top of `feat/web-form` locally that haven't been pushed, you won't see them.
- It does not run `sm restack`. Branches are checked out exactly as they exist on origin, even if origin's chain is "wrong" relative to current trunk.

## Cleanup after review

When you're done reviewing, you can drop the stack from your local repo:

```bash
sm checkout main
git branch -D feat/web-form feat/api-endpoints feat/db-schema
```

Or use [`sm untrack --reparent`](/commands/untrack) if you want to keep the branches but drop them from `sm`'s graph.

## See also

- [`sm get`](/commands/get) — flag reference.
- [`sm submit`](/commands/submit) — the inverse: push your own stack as PRs.
- [Concepts → Parent metadata](/concepts/parent-metadata).
