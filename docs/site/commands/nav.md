# sm up / down / top / bottom

Walk the stack relative to the current branch. Four small commands for the four directions.

## Synopses

```
sm up     [--first]
sm down   [--first]
sm top    [--first]
sm bottom [--first]
```

| Command | Direction |
|---|---|
| `sm up` | Move HEAD to the first child of the current branch. |
| `sm down` | Move HEAD to the parent of the current branch. |
| `sm top` | Walk up to a leaf of the current sub-stack. |
| `sm bottom` | Walk down to the branch sitting directly on trunk. |

## The `--first` flag

When the walk hits a fork (a branch with multiple children), the directional commands error out by default:

```
multiple children — pass --first or use `sm checkout` to choose
```

`--first` resolves the fork by taking the alphabetically first branch. Pass it when you don't care which fork:

```bash
sm top --first
```

## Examples

```
main
└── feat/api
    ├── feat/api-tests
    └── feat/api-docs
```

From `feat/api`:

```bash
sm up               # error: ambiguous fork
sm up --first       # → feat/api-docs (alphabetical)
sm checkout feat/api-tests   # explicit
```

From `feat/api-tests`:

```bash
sm down             # → feat/api
sm bottom           # → feat/api  (the branch sitting directly on trunk)
sm bottom; sm down  # → main
```

## What it does

For each direction, looks up the relationship in `sm`'s metadata, runs `git switch`, and prints the new branch.

## See also

- [`sm checkout`](./checkout) — direct switch or interactive picker.
- [`sm log`](./log) — see where you are in the tree.
