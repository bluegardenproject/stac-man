package service

import (
	"strings"
	"testing"

	"github.com/bluegardenproject/stac-man/internal/git"
)

func makeCommits(shas ...string) []git.Commit {
	out := make([]git.Commit, len(shas))
	for i, s := range shas {
		out[i] = git.Commit{SHA: s, Subject: "subject for " + s}
	}
	return out
}

func TestValidateMappingsHappyPath(t *testing.T) {
	commits := makeCommits("a1", "a2", "b1")
	mappings := []SplitMapping{
		{Name: "feat-a", Commits: []string{"a1", "a2"}},
		{Name: "feat-b", Commits: []string{"b1"}},
	}
	if err := validateMappings(mappings, commits); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateMappingsRejectsMissingCommit(t *testing.T) {
	commits := makeCommits("a1", "a2")
	mappings := []SplitMapping{
		{Name: "feat-a", Commits: []string{"a1"}},
		{Name: "feat-b", Commits: []string{"unknown"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "not part of the branch") {
		t.Fatalf("expected unknown-commit error, got %v", err)
	}
}

func TestValidateMappingsRejectsDuplicateCommit(t *testing.T) {
	commits := makeCommits("a1", "a2")
	mappings := []SplitMapping{
		{Name: "feat-a", Commits: []string{"a1"}},
		{Name: "feat-b", Commits: []string{"a1"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "multiple mappings") {
		t.Fatalf("expected duplicate-commit error, got %v", err)
	}
}

func TestValidateMappingsRequiresFullCoverage(t *testing.T) {
	commits := makeCommits("a1", "a2", "a3")
	mappings := []SplitMapping{
		{Name: "feat-a", Commits: []string{"a1"}},
		{Name: "feat-b", Commits: []string{"a2"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "branch has 3") {
		t.Fatalf("expected coverage error, got %v", err)
	}
}

func TestValidateMappingsEnforcesTopoOrder(t *testing.T) {
	commits := makeCommits("a1", "a2")
	// mapping puts older commit second — topologically out of order.
	mappings := []SplitMapping{
		{Name: "feat-b", Commits: []string{"a2"}},
		{Name: "feat-a", Commits: []string{"a1"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "topological") {
		t.Fatalf("expected topo-order error, got %v", err)
	}
}

func TestValidateMappingsRejectsDuplicateNames(t *testing.T) {
	commits := makeCommits("a1", "a2")
	mappings := []SplitMapping{
		{Name: "feat-x", Commits: []string{"a1"}},
		{Name: "feat-x", Commits: []string{"a2"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "duplicate branch name") {
		t.Fatalf("expected duplicate-name error, got %v", err)
	}
}

func TestValidateMappingsNeedsTwo(t *testing.T) {
	commits := makeCommits("a1")
	mappings := []SplitMapping{
		{Name: "feat-a", Commits: []string{"a1"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "two mappings") {
		t.Fatalf("expected two-mappings error, got %v", err)
	}
}

func TestValidateMappingsRejectsInvalidNames(t *testing.T) {
	commits := makeCommits("a1", "a2")
	mappings := []SplitMapping{
		{Name: "feat a", Commits: []string{"a1"}},
		{Name: "feat-b", Commits: []string{"a2"}},
	}
	err := validateMappings(mappings, commits)
	if err == nil || !strings.Contains(err.Error(), "invalid branch name") {
		t.Fatalf("expected invalid-name error, got %v", err)
	}
}

func TestAutoMappingsOnePerCommit(t *testing.T) {
	commits := []git.Commit{
		{SHA: "aaaa1", Subject: "Add foo"},
		{SHA: "bbbb2", Subject: "Fix bar"},
	}
	got := autoMappings(commits)
	if len(got) != 2 {
		t.Fatalf("got %d mappings", len(got))
	}
	if got[0].Name != "add-foo" || got[1].Name != "fix-bar" {
		t.Fatalf("slug names wrong: %+v", got)
	}
	if len(got[0].Commits) != 1 || got[0].Commits[0] != "aaaa1" {
		t.Fatalf("first mapping commits wrong: %+v", got[0].Commits)
	}
}

func TestAutoMappingsHandlesNameCollision(t *testing.T) {
	commits := []git.Commit{
		{SHA: "1", Subject: "fix"},
		{SHA: "2", Subject: "fix"},
		{SHA: "3", Subject: "fix"},
	}
	got := autoMappings(commits)
	seen := map[string]bool{}
	for _, m := range got {
		if seen[m.Name] {
			t.Fatalf("duplicate auto name %q", m.Name)
		}
		seen[m.Name] = true
	}
}

func TestSlugifyBasic(t *testing.T) {
	cases := map[string]string{
		"Add new feature": "add-new-feature",
		"  spaced  out  ": "spaced-out",
		"feat: thing":     "feat-thing",
		"":                "",
		"!!!":             "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Fatalf("slugify(%q): got %q want %q", in, got, want)
		}
	}
}

func TestParseRangesAcceptsValidSpecs(t *testing.T) {
	cases := []struct {
		spec string
		want []int
	}{
		{"1", []int{0}},
		{"1-3", []int{0, 1, 2}},
		{"1,3", []int{0, 2}},
		{"1-2,4", []int{0, 1, 3}},
	}
	for _, tc := range cases {
		got, err := ParseRanges(tc.spec, 5)
		if err != nil {
			t.Fatalf("ParseRanges(%q): %v", tc.spec, err)
		}
		if !equalInts(got, tc.want) {
			t.Fatalf("ParseRanges(%q): got %v want %v", tc.spec, got, tc.want)
		}
	}
}

func TestParseRangesRejectsBadInput(t *testing.T) {
	bad := []string{"0", "6", "3-1", "abc", "1-x"}
	for _, spec := range bad {
		if _, err := ParseRanges(spec, 5); err == nil {
			t.Fatalf("expected ParseRanges(%q) to fail", spec)
		}
	}
}

func equalInts(a, b []int) bool {
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
