package gh

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Client provides typed accessors over a Runner.
type Client struct {
	r Runner
}

// New returns a Client backed by ExecRunner rooted at dir.
func New(dir string) *Client {
	return &Client{r: ExecRunner{Dir: dir}}
}

// NewWithRunner is the seam for unit tests.
func NewWithRunner(r Runner) *Client {
	return &Client{r: r}
}

// RepoInfo identifies the GitHub repository the working directory
// belongs to.
type RepoInfo struct {
	Owner         string
	Name          string
	DefaultBranch string
}

// CurrentRepo returns the repo info for the current working directory
// using `gh repo view`. The owner/name pair is what every PR-mutating
// call needs.
func (c *Client) CurrentRepo(ctx context.Context) (RepoInfo, error) {
	out, _, err := c.r.Run(ctx, "repo", "view", "--json", "owner,name,defaultBranchRef")
	if err != nil {
		return RepoInfo{}, err
	}
	var raw struct {
		Owner             struct{ Login string } `json:"owner"`
		Name              string                  `json:"name"`
		DefaultBranchRef  struct{ Name string }  `json:"defaultBranchRef"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return RepoInfo{}, fmt.Errorf("parsing gh repo view: %w", err)
	}
	return RepoInfo{
		Owner:         raw.Owner.Login,
		Name:          raw.Name,
		DefaultBranch: raw.DefaultBranchRef.Name,
	}, nil
}

// PRState mirrors the subset of GitHub PR fields stac-man needs in
// `sm log` and `sm sync`.
type PRState string

const (
	PRStateOpen   PRState = "OPEN"
	PRStateMerged PRState = "MERGED"
	PRStateClosed PRState = "CLOSED"
)

// PR is a flattened PR record. We deliberately don't expose the full
// gh schema — the rest of the codebase only cares about these fields.
type PR struct {
	Number  int
	Title   string
	State   PRState
	IsDraft bool
	URL     string
	Base    string // base branch on GitHub
	Head    string // head branch on GitHub
}

// PRForBranch returns the most recent PR whose head matches branch.
// ok=false when there is no PR.
func (c *Client) PRForBranch(ctx context.Context, branch string) (PR, bool, error) {
	out, _, err := c.r.Run(ctx, "pr", "list",
		"--head", branch,
		"--state", "all",
		"--limit", "1",
		"--json", "number,title,state,isDraft,url,baseRefName,headRefName",
	)
	if err != nil {
		return PR{}, false, err
	}
	var raw []struct {
		Number      int    `json:"number"`
		Title       string `json:"title"`
		State       string `json:"state"`
		IsDraft     bool   `json:"isDraft"`
		URL         string `json:"url"`
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return PR{}, false, fmt.Errorf("parsing gh pr list: %w", err)
	}
	if len(raw) == 0 {
		return PR{}, false, nil
	}
	r := raw[0]
	return PR{
		Number:  r.Number,
		Title:   r.Title,
		State:   PRState(strings.ToUpper(r.State)),
		IsDraft: r.IsDraft,
		URL:     r.URL,
		Base:    r.BaseRefName,
		Head:    r.HeadRefName,
	}, true, nil
}

// PRsForBranches batches PR lookups across every branch in the slice.
// Returns a map keyed by head branch name; branches with no PR are
// simply absent from the map. Used by `sm log` to avoid one gh call
// per branch.
func (c *Client) PRsForBranches(ctx context.Context, branches []string) (map[string]PR, error) {
	if len(branches) == 0 {
		return map[string]PR{}, nil
	}
	// gh pr list does not support `--head` repeated, so we issue one
	// call per branch but still use a single `gh` invocation per call.
	// A future optimization can use the search API, but for v1 simple
	// is better.
	out := make(map[string]PR, len(branches))
	for _, b := range branches {
		pr, ok, err := c.PRForBranch(ctx, b)
		if err != nil {
			return nil, fmt.Errorf("looking up PR for %s: %w", b, err)
		}
		if ok {
			out[b] = pr
		}
	}
	return out, nil
}

// CreatePROptions configures CreatePR.
type CreatePROptions struct {
	Title string
	Body  string
	Head  string // head branch on GitHub
	Base  string // base branch on GitHub
	Draft bool
}

// CreatePR opens a new pull request using `gh pr create`. The branch
// must already be pushed to origin. Returns the created PR number.
func (c *Client) CreatePR(ctx context.Context, opts CreatePROptions) (int, error) {
	args := []string{"pr", "create",
		"--title", opts.Title,
		"--body", opts.Body,
		"--head", opts.Head,
		"--base", opts.Base,
	}
	if opts.Draft {
		args = append(args, "--draft")
	}
	out, _, err := c.r.Run(ctx, args...)
	if err != nil {
		return 0, err
	}
	// `gh pr create` prints the new PR URL (e.g. ".../pull/42") on the
	// last non-empty line. Parse the trailing number out of it rather
	// than running an extra `gh pr view`.
	num, perr := parsePRNumberFromURL(out)
	if perr != nil {
		return 0, fmt.Errorf("parsing gh pr create output %q: %w", strings.TrimSpace(out), perr)
	}
	return num, nil
}

// EditPROptions configures EditPR. Only non-empty fields are sent to gh.
type EditPROptions struct {
	Title string
	Body  string
	Base  string
}

// EditPR updates an existing PR. Use Base to retarget when a branch
// has been re-parented locally.
func (c *Client) EditPR(ctx context.Context, number int, opts EditPROptions) error {
	if opts.Title == "" && opts.Body == "" && opts.Base == "" {
		return nil
	}
	args := []string{"pr", "edit", fmt.Sprintf("%d", number)}
	if opts.Title != "" {
		args = append(args, "--title", opts.Title)
	}
	if opts.Body != "" {
		args = append(args, "--body", opts.Body)
	}
	if opts.Base != "" {
		args = append(args, "--base", opts.Base)
	}
	_, _, err := c.r.Run(ctx, args...)
	return err
}

// parsePRNumberFromURL pulls the trailing integer off a github.com PR
// URL. Tolerates trailing whitespace and surrounding text since
// different gh versions print slightly differently.
func parsePRNumberFromURL(s string) (int, error) {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "/pull/") {
			continue
		}
		idx := strings.LastIndex(line, "/")
		if idx < 0 || idx == len(line)-1 {
			continue
		}
		var n int
		_, err := fmt.Sscanf(line[idx+1:], "%d", &n)
		if err == nil && n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("no PR URL found")
}
