package gh

import (
	"context"
	"errors"
	"testing"
)

type fakeRunner struct {
	calls     [][]string
	responses map[string]fakeResponse
	fallback  fakeResponse
}

type fakeResponse struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if r, ok := f.responses[joinArgs(args)]; ok {
		return r.stdout, r.stderr, r.err
	}
	return f.fallback.stdout, f.fallback.stderr, f.fallback.err
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

func TestCurrentRepo(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"repo view --json owner,name,defaultBranchRef": {
				stdout: `{"owner":{"login":"acme"},"name":"widgets","defaultBranchRef":{"name":"main"}}`,
			},
		},
	}
	c := NewWithRunner(r)
	got, err := c.CurrentRepo(context.Background())
	if err != nil {
		t.Fatalf("CurrentRepo: %v", err)
	}
	want := RepoInfo{Owner: "acme", Name: "widgets", DefaultBranch: "main"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestPRForBranchExists(t *testing.T) {
	r := &fakeRunner{
		fallback: fakeResponse{
			stdout: `[{"number":42,"title":"Add login","state":"OPEN","isDraft":false,"url":"https://github.com/acme/widgets/pull/42","baseRefName":"main","headRefName":"feat/login"}]`,
		},
	}
	c := NewWithRunner(r)
	pr, ok, err := c.PRForBranch(context.Background(), "feat/login")
	if err != nil {
		t.Fatalf("PRForBranch: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if pr.Number != 42 || pr.State != PRStateOpen || pr.IsDraft {
		t.Fatalf("unexpected PR: %+v", pr)
	}
}

func TestPRForBranchMissing(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{stdout: "[]"}}
	c := NewWithRunner(r)
	_, ok, err := c.PRForBranch(context.Background(), "nope")
	if err != nil {
		t.Fatalf("PRForBranch: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false")
	}
}

func TestCreatePRParsesURL(t *testing.T) {
	r := &fakeRunner{
		fallback: fakeResponse{
			stdout: "Creating pull request for feat/login into main in acme/widgets\n\nhttps://github.com/acme/widgets/pull/123\n",
		},
	}
	c := NewWithRunner(r)
	num, err := c.CreatePR(context.Background(), CreatePROptions{
		Title: "Add login",
		Body:  "Implements basic auth.",
		Head:  "feat/login",
		Base:  "main",
	})
	if err != nil {
		t.Fatalf("CreatePR: %v", err)
	}
	if num != 123 {
		t.Fatalf("got PR %d, want 123", num)
	}
	// Check the args we sent — title, body, head, base ordering matters
	// for `gh pr create`.
	got := r.calls[0]
	for _, want := range []string{"pr", "create", "--title", "Add login", "--head", "feat/login", "--base", "main"} {
		if !contains(got, want) {
			t.Fatalf("expected %q in args, got %v", want, got)
		}
	}
}

func TestCreatePRDraftFlag(t *testing.T) {
	r := &fakeRunner{
		fallback: fakeResponse{stdout: "https://github.com/acme/widgets/pull/9\n"},
	}
	c := NewWithRunner(r)
	if _, err := c.CreatePR(context.Background(), CreatePROptions{
		Title: "wip", Head: "feat/wip", Base: "main", Draft: true,
	}); err != nil {
		t.Fatalf("CreatePR: %v", err)
	}
	if !contains(r.calls[0], "--draft") {
		t.Fatalf("expected --draft in args, got %v", r.calls[0])
	}
}

func TestEditPRSkipsWhenEmpty(t *testing.T) {
	r := &fakeRunner{}
	c := NewWithRunner(r)
	if err := c.EditPR(context.Background(), 1, EditPROptions{}); err != nil {
		t.Fatalf("EditPR: %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("expected no calls when nothing to edit, got %d", len(r.calls))
	}
}

func TestEditPRSendsRequestedFields(t *testing.T) {
	r := &fakeRunner{}
	c := NewWithRunner(r)
	if err := c.EditPR(context.Background(), 5, EditPROptions{Base: "feat/parent"}); err != nil {
		t.Fatalf("EditPR: %v", err)
	}
	got := r.calls[0]
	if !contains(got, "--base") || !contains(got, "feat/parent") {
		t.Fatalf("expected --base feat/parent, got %v", got)
	}
}

func TestParsePRNumberFromURLVariants(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"https://github.com/o/r/pull/7\n", 7},
		{"some prefix line\nhttps://github.com/o/r/pull/123\n", 123},
		{"  https://github.com/o/r/pull/9999  ", 9999},
	}
	for _, tc := range cases {
		got, err := parsePRNumberFromURL(tc.in)
		if err != nil {
			t.Errorf("parsePRNumberFromURL(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parsePRNumberFromURL(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestParsePRNumberFromURLNoMatch(t *testing.T) {
	_, err := parsePRNumberFromURL("nothing here\n")
	if err == nil {
		t.Fatalf("expected error for input without a PR URL")
	}
}

func TestCurrentRepoSurfacesGhError(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: errors.New("boom")}}
	c := NewWithRunner(r)
	if _, err := c.CurrentRepo(context.Background()); err == nil {
		t.Fatalf("expected error to bubble up")
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
