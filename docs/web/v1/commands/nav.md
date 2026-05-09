> [!WARNING]
> You are viewing the v1 docs. v2 is the latest version and changes GitHub status behavior. See [v2 docs](/).

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

From `feat/api`, walking up hits the fork. Bare `sm up` errors:

```bash
sm up
```

`--first` takes the alphabetically-first child (`feat/api-docs`):

```bash
sm up --first
```

Or pick the other fork explicitly:

```bash
sm checkout feat/api-tests
```

From `feat/api-tests`, walk one step down to the parent:

```bash
sm down
```

Or jump to the branch sitting directly on trunk (here that's `feat/api` again):

```bash
sm bottom
```

To get all the way to trunk itself, do `bottom` then one more `down`:

```bash
sm bottom
sm down
```

## What it does

For each direction, looks up the relationship in `sm`'s metadata, runs `git switch`, and prints the new branch.

## See also

- [`sm checkout`](./checkout) — direct switch or interactive picker.
- [`sm log`](./log) — see where you are in the tree.
