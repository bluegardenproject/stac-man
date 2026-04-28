# sm checkout

Switch HEAD to a tracked branch. Without an argument and on a TTY, opens an interactive picker.

Alias: `sm co`.

## Synopsis

```
sm checkout [branch]
```

## Examples

Direct switch:

```bash
sm checkout feat/auth-handlers
```

Interactive picker (TTY only) — arrow keys, Enter to choose, Esc to cancel:

```bash
sm checkout
```

Pipe-friendly fallback — when stdin or stdout is not a TTY, the same tree is printed instead of launching the picker:

```bash
sm checkout | grep feat
```

## What it does

1. With an argument: validates the branch is tracked (or is trunk) and runs `git switch`.
2. Without an argument: builds the same tree `sm log` would render and either:
   - Launches a Bubble Tea picker on a TTY, or
   - Prints the tree statically when stdin/stdout isn't a TTY.

## Tab completion

After running `sm completion <shell>` (see [`sm completion`](./completion)), `sm checkout <TAB>` completes against tracked branches plus trunk.

## See also

- [`sm up` / `down` / `top` / `bottom`](./nav) — directional navigation that reads naturally for stack walks.
- [`sm log`](./log) — see the tree without switching.
