package git

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// fakeRunner is a deterministic Runner used by Client tests.
//
// Calls is appended in order so tests can assert which git invocations
// the Client made. Responses are looked up by exact argument tuple
// (joined with a space); use this rather than regex matching so tests
// fail loudly when the Client changes its argv.
type fakeRunner struct {
	calls     [][]string
	responses map[string]fakeResponse
	// fallback is returned when no responses entry matches; useful when
	// a test only cares about a subset of the calls.
	fallback fakeResponse
}

type fakeResponse struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) Run(_ context.Context, args ...string) (string, string, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	key := join(args)
	if r, ok := f.responses[key]; ok {
		return r.stdout, r.stderr, r.err
	}
	return f.fallback.stdout, f.fallback.stderr, f.fallback.err
}

func join(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

// exitErr returns a real *exec.ExitError with the requested exit code,
// so tests can drive the same ExitErrorOf walk that production uses.
// We can't construct *exec.ExitError directly (its internals are
// unexported) so we shell out to a guaranteed-to-fail helper.
func exitErr(code int) error {
	// Use exec.Command with a guaranteed-to-fail program to obtain a
	// real *exec.ExitError so ExitErrorOf works as in production.
	cmd := exec.Command("false")
	if code != 1 {
		// `false` always exits 1; for other codes we use sh -c.
		cmd = exec.Command("sh", "-c", "exit "+itoa(code))
	}
	err := cmd.Run()
	if err == nil {
		// Should never happen — protect tests from silent regressions.
		panic("expected non-nil error from forced-failure command")
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) {
		panic("expected *exec.ExitError from forced-failure command")
	}
	return ee
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestExecRunnerOverridesEditorEnv pins B4: every `git` invocation
// from sm must carry GIT_EDITOR=: (and EDITOR=: as a legacy fallback)
// so non-interactive contexts — most importantly `git rebase --continue`
// triggered by `sm continue` — don't fail with "Terminal is dumb, but
// EDITOR unset".
func TestExecRunnerOverridesEditorEnv(t *testing.T) {
	r := ExecRunner{Dir: t.TempDir()}
	cmd := r.command(context.Background(), "status", "--porcelain")

	if got, want := cmd.Args[0], "git"; got != want {
		t.Fatalf("argv[0] = %q, want %q", got, want)
	}
	wantKeys := map[string]string{"GIT_EDITOR": ":", "EDITOR": ":"}
	got := map[string]string{}
	for _, kv := range cmd.Env {
		for k := range wantKeys {
			if strings.HasPrefix(kv, k+"=") {
				got[k] = strings.TrimPrefix(kv, k+"=")
			}
		}
	}
	for k, want := range wantKeys {
		if got[k] != want {
			t.Fatalf("env %s = %q, want %q (full env had %d entries)", k, got[k], want, len(cmd.Env))
		}
	}
}

func TestCurrentBranch(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"symbolic-ref --short HEAD": {stdout: "feature/a\n"},
		},
	}
	c := NewWithRunner(r)
	got, err := c.CurrentBranch(context.Background())
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "feature/a" {
		t.Fatalf("got %q, want %q", got, "feature/a")
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
}

func TestBranchExists(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"exists", nil, true},
		{"missing", exitErr(1), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &fakeRunner{fallback: fakeResponse{err: tc.err}}
			c := NewWithRunner(r)
			got, err := c.BranchExists(context.Background(), "feat/x")
			if err != nil {
				t.Fatalf("BranchExists: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsClean(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"status --porcelain --untracked-files=no": {stdout: ""},
		},
	}
	c := NewWithRunner(r)
	clean, err := c.IsClean(context.Background())
	if err != nil {
		t.Fatalf("IsClean: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean tree")
	}
}

func TestConfigGetMissingKey(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: exitErr(1)}}
	c := NewWithRunner(r)
	val, ok, err := c.ConfigGet(context.Background(), "branch.foo.stac-man-parent")
	if err != nil {
		t.Fatalf("ConfigGet: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false for missing key")
	}
	if val != "" {
		t.Fatalf("expected empty value, got %q", val)
	}
}

func TestConfigUnsetIgnoresMissing(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: exitErr(5)}}
	c := NewWithRunner(r)
	if err := c.ConfigUnset(context.Background(), "branch.foo.bar"); err != nil {
		t.Fatalf("ConfigUnset returned %v, want nil for exit-5", err)
	}
}

func TestConfigListPrefixFilter(t *testing.T) {
	out := "core.bare=false\n" +
		"branch.feat-a.stac-man-parent=main\n" +
		"branch.feat-a.stac-man-parent-sha=abc123\n" +
		"branch.feat-b.stac-man-parent=feat-a\n" +
		"user.name=alice\n"
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"config --local --list": {stdout: out},
		},
	}
	c := NewWithRunner(r)
	got, err := c.ConfigList(context.Background(), "branch.")
	if err != nil {
		t.Fatalf("ConfigList: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(got), got)
	}
	if got["branch.feat-a.stac-man-parent"] != "main" {
		t.Fatalf("unexpected value: %v", got)
	}
}

func TestPushUsesForceWithLease(t *testing.T) {
	r := &fakeRunner{}
	c := NewWithRunner(r)
	if err := c.Push(context.Background(), "feat-a", true); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call")
	}
	got := r.calls[0]
	want := []string{"push", "--set-upstream", "--force-with-lease", "origin", "feat-a"}
	if !equalSlices(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestPushPlain(t *testing.T) {
	r := &fakeRunner{}
	c := NewWithRunner(r)
	if err := c.Push(context.Background(), "feat-a", false); err != nil {
		t.Fatalf("Push: %v", err)
	}
	got := r.calls[0]
	want := []string{"push", "--set-upstream", "origin", "feat-a"}
	if !equalSlices(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestLocalBranches(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"for-each-ref --format=%(refname:short) refs/heads/": {stdout: "main\nfeat-a\nfeat-b\n"},
		},
	}
	c := NewWithRunner(r)
	got, err := c.LocalBranches(context.Background())
	if err != nil {
		t.Fatalf("LocalBranches: %v", err)
	}
	want := []string{"main", "feat-a", "feat-b"}
	if !equalSlices(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestRemoteMatchesLocalEqual locks in B10's idempotent-push seam:
// when local and `refs/remotes/origin/<branch>` resolve to the same
// SHA, the helper reports true so `sm submit`'s push loop knows to
// skip a redundant force-with-lease.
func TestRemoteMatchesLocalEqual(t *testing.T) {
	const sha = "deadbeefcafebabedeadbeefcafebabedeadbeef"
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"rev-parse --verify feat-a^{commit}":                     {stdout: sha + "\n"},
			"rev-parse --verify refs/remotes/origin/feat-a^{commit}": {stdout: sha + "\n"},
		},
	}
	c := NewWithRunner(r)
	got, err := c.RemoteMatchesLocal(context.Background(), "feat-a")
	if err != nil {
		t.Fatalf("RemoteMatchesLocal: %v", err)
	}
	if !got {
		t.Fatalf("RemoteMatchesLocal = false, want true (same SHAs)")
	}
}

func TestRemoteMatchesLocalDiverged(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"rev-parse --verify feat-a^{commit}":                     {stdout: "aaaa\n"},
			"rev-parse --verify refs/remotes/origin/feat-a^{commit}": {stdout: "bbbb\n"},
		},
	}
	c := NewWithRunner(r)
	got, err := c.RemoteMatchesLocal(context.Background(), "feat-a")
	if err != nil {
		t.Fatalf("RemoteMatchesLocal: %v", err)
	}
	if got {
		t.Fatalf("RemoteMatchesLocal = true, want false (different SHAs)")
	}
}

// A brand-new branch that has never been pushed has no
// `refs/remotes/origin/<branch>` ref. rev-parse exits non-zero in
// that case; the helper must treat it as "not in sync" without
// bubbling the error so the caller falls through to the actual
// push.
func TestRemoteMatchesLocalMissingRemoteRef(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"rev-parse --verify feat-new^{commit}":                     {stdout: "aaaa\n"},
			"rev-parse --verify refs/remotes/origin/feat-new^{commit}": {err: exitErr(1)},
		},
	}
	c := NewWithRunner(r)
	got, err := c.RemoteMatchesLocal(context.Background(), "feat-new")
	if err != nil {
		t.Fatalf("RemoteMatchesLocal returned %v, want nil for missing remote ref", err)
	}
	if got {
		t.Fatalf("RemoteMatchesLocal = true, want false when remote ref missing")
	}
}

func TestRebaseArgs(t *testing.T) {
	r := &fakeRunner{}
	c := NewWithRunner(r)
	if err := c.Rebase(context.Background(), "newParentSha", "oldParentSha", "feat-b"); err != nil {
		t.Fatalf("Rebase: %v", err)
	}
	want := []string{"rebase", "--onto", "newParentSha", "oldParentSha", "feat-b"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("got %v, want %v", r.calls[0], want)
	}
}

// TestConflictPathsNoneIsEmptySlice locks the contract the cockpit's
// resolver relies on: a clean index returns an empty slice (not nil
// + a "no conflicts" error). The caller can then render
// "no conflicts" without distinguishing between absence and failure.
func TestConflictPathsNoneIsEmptySlice(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"diff --name-only --diff-filter=U": {stdout: ""},
		},
	}
	c := NewWithRunner(r)
	got, err := c.ConflictPaths(context.Background())
	if err != nil {
		t.Fatalf("ConflictPaths: %v", err)
	}
	if got == nil {
		t.Fatalf("ConflictPaths returned nil, want empty slice (cockpit treats nil as failure)")
	}
	if len(got) != 0 {
		t.Fatalf("ConflictPaths = %v, want empty", got)
	}
}

// TestConflictPathsParsesMultipleLines pins the parser: one path per
// line, blank lines skipped, surrounding whitespace stripped — so a
// terminal trailing newline (which `git diff` always emits) does
// not produce a phantom empty entry.
func TestConflictPathsParsesMultipleLines(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"diff --name-only --diff-filter=U": {stdout: "internal/foo.go\ninternal/bar.go\n"},
		},
	}
	c := NewWithRunner(r)
	got, err := c.ConflictPaths(context.Background())
	if err != nil {
		t.Fatalf("ConflictPaths: %v", err)
	}
	want := []string{"internal/foo.go", "internal/bar.go"}
	if !equalSlices(got, want) {
		t.Fatalf("ConflictPaths = %v, want %v", got, want)
	}
}

// TestConflictPathsSurfacesGitErrors guarantees a real git failure
// (missing repo, broken index) bubbles up rather than collapsing to
// "no conflicts" — silent collapse here would hide a stuck restack
// from the resolver UI.
func TestConflictPathsSurfacesGitErrors(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: exitErr(128)}}
	c := NewWithRunner(r)
	if _, err := c.ConflictPaths(context.Background()); err == nil {
		t.Fatalf("ConflictPaths returned nil error, want propagation of git failure")
	}
}

// TestDiffArgsAndPassthrough pins two contracts the cockpit's diff
// viewer relies on: the exact argv (range syntax + --no-color) and
// the fact that the body is returned verbatim. Hunk whitespace is
// position-sensitive, so even a single TrimSpace would silently
// corrupt blank-line context lines.
func TestDiffArgsAndPassthrough(t *testing.T) {
	body := "diff --git a/foo b/foo\n--- a/foo\n+++ b/foo\n@@ -1,1 +1,1 @@\n-old\n+new\n"
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"diff --no-color main..feat-a": {stdout: body},
		},
	}
	c := NewWithRunner(r)
	got, err := c.Diff(context.Background(), "main", "feat-a")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if got != body {
		t.Fatalf("Diff body got %q, want %q (verbatim passthrough required)", got, body)
	}
	if len(r.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(r.calls))
	}
	want := []string{"diff", "--no-color", "main..feat-a"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("argv = %v, want %v", r.calls[0], want)
	}
}

// TestDiffEmptyIsNotAnError covers the "branches at the same commit"
// case: git exits 0 with empty stdout and the helper should report
// that as a clean empty result rather than a failure.
func TestDiffEmptyIsNotAnError(t *testing.T) {
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"diff --no-color main..feat-a": {stdout: ""},
		},
	}
	c := NewWithRunner(r)
	got, err := c.Diff(context.Background(), "main", "feat-a")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if got != "" {
		t.Fatalf("Diff = %q, want empty", got)
	}
}

// TestDiffSurfaceErrorsFromGit guarantees the helper does not swallow
// genuine failures (bad ref, repo missing, etc.). Without this, the
// cockpit could silently render an empty diff for an invalid range.
func TestDiffSurfaceErrorsFromGit(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: exitErr(128)}}
	c := NewWithRunner(r)
	if _, err := c.Diff(context.Background(), "main", "no-such-branch"); err == nil {
		t.Fatalf("Diff returned nil error, want propagation of git failure")
	}
}

// TestShowArgsAndPassthrough pins the per-commit viewer contract:
// `git show --no-color <ref>` and the body returned verbatim so the
// commit metadata header (author, date, message) survives intact.
func TestShowArgsAndPassthrough(t *testing.T) {
	body := "commit deadbeef\nAuthor: Foo <foo@bar>\nDate: now\n\n    subject\n\n    body\n\ndiff --git a/x b/x\n"
	r := &fakeRunner{
		responses: map[string]fakeResponse{
			"show --no-color deadbeef": {stdout: body},
		},
	}
	c := NewWithRunner(r)
	got, err := c.Show(context.Background(), "deadbeef")
	if err != nil {
		t.Fatalf("Show: %v", err)
	}
	if got != body {
		t.Fatalf("Show body got %q, want %q (verbatim passthrough required)", got, body)
	}
	want := []string{"show", "--no-color", "deadbeef"}
	if !equalSlices(r.calls[0], want) {
		t.Fatalf("argv = %v, want %v", r.calls[0], want)
	}
}

// TestShowSurfaceErrorsFromGit mirrors the Diff equivalent: an
// invalid ref must propagate so the cockpit can show a real error.
func TestShowSurfaceErrorsFromGit(t *testing.T) {
	r := &fakeRunner{fallback: fakeResponse{err: exitErr(128)}}
	c := NewWithRunner(r)
	if _, err := c.Show(context.Background(), "no-such-sha"); err == nil {
		t.Fatalf("Show returned nil error, want propagation of git failure")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
