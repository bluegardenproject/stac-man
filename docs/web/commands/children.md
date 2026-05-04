# sm children

List the direct children of a branch.

## Synopsis

```
sm children [branch]
```

If no branch is given, lists children of the current branch.

## Examples

```bash
sm children
# feat/auth-handlers
# feat/auth-tests
```

If the branch has no tracked children, prints `(no children)`:

```bash
sm children feat/auth-tests
# (no children)
```

Pipeline-friendly — one branch per line, no decoration:

```bash
sm children | xargs -L1 sm show
```

## What it does

Reads the in-memory `Graph` and emits the names of branches whose `stac-man-parent` is the named branch. Untracked branches are not included.

## See also

- [`sm parent`](./parent) — the inverse direction.
- [`sm log`](./log) — the whole tree.
- [`sm show`](./show) — children appear in the human-readable summary too.
