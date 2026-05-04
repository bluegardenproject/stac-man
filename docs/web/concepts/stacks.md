# Stacks

A **stack** is a chain of dependent branches, each with its own pull request, where each PR's base is the branch immediately below it instead of trunk.

```
main
└── feat/db-schema           (PR #41 → main)
    └── feat/api-endpoints   (PR #42 → feat/db-schema)
        └── feat/web-form    (PR #43 → feat/api-endpoints)
```

Reviewers see three small PRs they can approve top-down, instead of one 1,200-line change. As the bottom PR merges, the next one's base flips to trunk.

## Why bother

- **Faster reviews.** A 200-line PR is reviewed; a 2,000-line PR is rubber-stamped or, worse, rejected as too big.
- **Parallel work.** You don't have to wait for review to start the next layer.
- **Smaller blast radius.** Bisects, reverts, and conflict resolution operate on a single layer.

## What `sm` adds on top of git

Git itself has no concept of "this branch's parent is that other branch." `sm` records that relationship in `.git/config`:

```
branch.feat/api-endpoints.stac-man-parent = feat/db-schema
branch.feat/api-endpoints.stac-man-parent-sha = 4f1e2…
branch.feat/api-endpoints.stac-man-pr = 42
```

With those keys recorded, `sm` can:

- Walk the chain in topological order (`sm log`, `sm restack`, `sm sync`).
- Rebase descendants automatically when an ancestor moves (`sm modify`, `sm absorb`, `sm move`).
- Wire PR base branches correctly when you submit (`sm submit --stack`).
- Detect and report drift (`sm doctor`).

## Trees, not just chains

Stacks can branch. Two children can hang off the same parent:

```
main
└── feat/auth-models
    ├── feat/auth-handlers
    └── feat/auth-tests
```

Every `sm` command handles trees. The only restriction is that a branch has exactly one parent. If you need to graft work from two ancestors together, that's a merge commit, not a stacked-PR pattern, and `sm` won't try to reason about it.

## The shape of a workflow

Every `sm` workflow boils down to four verbs:

| Phase | Command | What it does |
|---|---|---|
| Create | `sm create <name>` | Branch + record parent. |
| Modify | `sm modify [-a] -m <msg>` | Commit (or amend) + restack descendants. |
| Restack | `sm restack` | Rebase chain after an upstream change. |
| Submit | `sm submit --stack` | Push + open/update PRs with bases wired. |

Read on for what [trunk](./trunk), [restack](./restack), and [sync](./sync) actually do.
