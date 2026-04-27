package git

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Client provides typed accessors over a Runner. Methods take a
// context.Context so a Ctrl+C cancels long-running git invocations.
type Client struct {
	r Runner
}

// New returns a Client backed by an ExecRunner rooted at dir. Pass an
// empty dir to inherit the parent process's CWD.
func New(dir string) *Client {
	return &Client{r: ExecRunner{Dir: dir}}
}

// NewWithRunner is the seam used by unit tests.
func NewWithRunner(r Runner) *Client {
	return &Client{r: r}
}

// ErrNotARepo is returned when a git operation runs outside a git
// working tree.
var ErrNotARepo = errors.New("not a git repository")

// IsRepo reports whether the working directory is inside a git work
// tree.
func (c *Client) IsRepo(ctx context.Context) (bool, error) {
	out, _, err := c.r.Run(ctx, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		// 128 = "fatal: not a git repository". Treat as a clean false
		// instead of bubbling the error up so callers can present a
		// friendlier message.
		if ExitErrorOf(err) == 128 {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

// GitDir returns the path to the .git directory for the current
// (possibly worktree-specific) repository.
func (c *Client) GitDir(ctx context.Context) (string, error) {
	out, _, err := c.r.Run(ctx, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CurrentBranch returns the short name of the branch HEAD is on, or
// an error if HEAD is detached.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, _, err := c.r.Run(ctx, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(out), nil
}

// RevParse resolves a ref to its commit SHA. Returns ErrNotARepo /
// nothing-found errors as a non-nil error so callers can decide.
func (c *Client) RevParse(ctx context.Context, ref string) (string, error) {
	out, _, err := c.r.Run(ctx, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// BranchExists reports whether a local branch with the given name
// exists.
func (c *Client) BranchExists(ctx context.Context, name string) (bool, error) {
	_, _, err := c.r.Run(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, nil
	}
	if ExitErrorOf(err) == 1 {
		return false, nil
	}
	return false, err
}

// MergeBase returns `git merge-base a b`.
func (c *Client) MergeBase(ctx context.Context, a, b string) (string, error) {
	out, _, err := c.r.Run(ctx, "merge-base", a, b)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// IsAncestor reports whether `ancestor` is reachable from `descendant`.
// Wraps `git merge-base --is-ancestor`.
func (c *Client) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	_, _, err := c.r.Run(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if ExitErrorOf(err) == 1 {
		return false, nil
	}
	return false, err
}

// IsClean reports whether the working tree has no staged or unstaged
// changes (porcelain output is empty). Untracked files are ignored.
func (c *Client) IsClean(ctx context.Context) (bool, error) {
	out, _, err := c.r.Run(ctx, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "", nil
}

// LocalBranches returns the names of all local branches.
func (c *Client) LocalBranches(ctx context.Context) ([]string, error) {
	out, _, err := c.r.Run(ctx, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	branches := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			branches = append(branches, l)
		}
	}
	return branches, nil
}

// CreateBranch creates a new branch at HEAD without checking it out.
func (c *Client) CreateBranch(ctx context.Context, name string) error {
	_, _, err := c.r.Run(ctx, "branch", name)
	return err
}

// Checkout switches HEAD to branch.
func (c *Client) Checkout(ctx context.Context, branch string) error {
	_, _, err := c.r.Run(ctx, "checkout", branch)
	return err
}

// CheckoutNew creates and checks out a new branch off HEAD.
func (c *Client) CheckoutNew(ctx context.Context, branch string) error {
	_, _, err := c.r.Run(ctx, "checkout", "-b", branch)
	return err
}

// DeleteBranch removes a local branch. Force deletes even if not merged.
func (c *Client) DeleteBranch(ctx context.Context, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, _, err := c.r.Run(ctx, "branch", flag, branch)
	return err
}

// AddAll stages all changes (`git add -A`).
func (c *Client) AddAll(ctx context.Context) error {
	_, _, err := c.r.Run(ctx, "add", "-A")
	return err
}

// Commit creates a commit with the given message. If amend is true the
// previous commit is replaced; if allowEmpty is true an empty commit
// won't fail.
func (c *Client) Commit(ctx context.Context, msg string, amend, allowEmpty bool) error {
	args := []string{"commit"}
	if amend {
		args = append(args, "--amend")
		if msg == "" {
			args = append(args, "--no-edit")
		}
	}
	if allowEmpty {
		args = append(args, "--allow-empty")
	}
	if msg != "" {
		args = append(args, "-m", msg)
	}
	_, _, err := c.r.Run(ctx, args...)
	return err
}

// MergeSquash runs `git merge --squash <branch>` against HEAD. Used
// by `sm fold` to collapse a branch's commits onto its parent without
// creating a merge commit.
func (c *Client) MergeSquash(ctx context.Context, branch string) error {
	_, _, err := c.r.Run(ctx, "merge", "--squash", branch)
	return err
}

// Rebase runs `git rebase --onto onto upstream branch`. Use this for
// stack restacks: `onto` is the parent's new tip, `upstream` is the
// parent's old tip, `branch` is the branch being moved.
func (c *Client) Rebase(ctx context.Context, onto, upstream, branch string) error {
	_, _, err := c.r.Run(ctx, "rebase", "--onto", onto, upstream, branch)
	return err
}

// RebaseInProgress reports whether a rebase is currently paused (e.g.
// after a conflict). Detected by the presence of .git/rebase-merge or
// .git/rebase-apply.
func (c *Client) RebaseInProgress(ctx context.Context) (bool, error) {
	gitDir, err := c.GitDir(ctx)
	if err != nil {
		return false, err
	}
	for _, sub := range []string{"rebase-merge", "rebase-apply"} {
		exists, err := pathExists(gitDir + "/" + sub)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

// RebaseContinue runs `git rebase --continue`.
func (c *Client) RebaseContinue(ctx context.Context) error {
	_, _, err := c.r.Run(ctx, "rebase", "--continue")
	return err
}

// RebaseAbort runs `git rebase --abort`.
func (c *Client) RebaseAbort(ctx context.Context) error {
	_, _, err := c.r.Run(ctx, "rebase", "--abort")
	return err
}

// ConfigGet reads a single git config value. Returns "" and false if
// the key isn't set.
func (c *Client) ConfigGet(ctx context.Context, key string) (string, bool, error) {
	out, _, err := c.r.Run(ctx, "config", "--local", "--get", key)
	if err != nil {
		// Exit 1 from `git config --get` means "key not found".
		if ExitErrorOf(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}

// ConfigSet writes a single git config value.
func (c *Client) ConfigSet(ctx context.Context, key, value string) error {
	_, _, err := c.r.Run(ctx, "config", "--local", key, value)
	return err
}

// ConfigUnset removes a git config key. Returns nil if the key didn't
// exist.
func (c *Client) ConfigUnset(ctx context.Context, key string) error {
	_, _, err := c.r.Run(ctx, "config", "--local", "--unset", key)
	if err != nil && ExitErrorOf(err) == 5 {
		// Exit 5 = "you try to unset an option which does not exist".
		return nil
	}
	return err
}

// ConfigList returns all `--local` config keys whose name starts with
// prefix, mapped to their values. Used to scan for `branch.*.stac-man-*`.
func (c *Client) ConfigList(ctx context.Context, prefix string) (map[string]string, error) {
	out, _, err := c.r.Run(ctx, "config", "--local", "--list")
	if err != nil {
		return nil, err
	}
	res := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		key, val := line[:eq], line[eq+1:]
		if prefix == "" || strings.HasPrefix(key, prefix) {
			res[key] = val
		}
	}
	return res, nil
}

// FetchAll runs `git fetch --all --prune`.
func (c *Client) FetchAll(ctx context.Context) error {
	_, _, err := c.r.Run(ctx, "fetch", "--all", "--prune")
	return err
}

// Pull runs `git pull --ff-only origin <branch>`. We refuse non-ff
// pulls so trunk updates never produce surprise merge commits.
func (c *Client) Pull(ctx context.Context, branch string) error {
	_, _, err := c.r.Run(ctx, "pull", "--ff-only", "origin", branch)
	return err
}

// CountCommitsAhead returns how many commits `branch` has that are
// not in `base`. Used by `sm sync` to detect branches whose work has
// been merged (count == 0 means fully absorbed by trunk).
func (c *Client) CountCommitsAhead(ctx context.Context, branch, base string) (int, error) {
	out, _, err := c.r.Run(ctx, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, nil
	}
	var n int
	if _, err := fmt.Sscanf(out, "%d", &n); err != nil {
		return 0, fmt.Errorf("parsing rev-list count %q: %w", out, err)
	}
	return n, nil
}

// Push pushes branch to origin. When forceLease is true it uses
// --force-with-lease (never plain --force) which is what `sm submit`
// needs after a restack rewrites history.
func (c *Client) Push(ctx context.Context, branch string, forceLease bool) error {
	args := []string{"push", "--set-upstream"}
	if forceLease {
		args = append(args, "--force-with-lease")
	}
	args = append(args, "origin", branch)
	_, _, err := c.r.Run(ctx, args...)
	return err
}

// SymbolicRef reads a symbolic ref (e.g. "refs/remotes/origin/HEAD").
// Returns "" and false if the ref isn't set.
func (c *Client) SymbolicRef(ctx context.Context, name string) (string, bool, error) {
	out, _, err := c.r.Run(ctx, "symbolic-ref", "--quiet", name)
	if err != nil {
		if ExitErrorOf(err) == 1 {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(out), true, nil
}
