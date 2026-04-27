package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBranchViewJSONShape(t *testing.T) {
	view := &BranchView{
		Branch:    "feat-a",
		Trunk:     "main",
		Tracked:   true,
		Parent:    "main",
		ParentSHA: "abc123",
		Tip:       "def456",
		Children:  []string{"feat-b", "feat-c"},
		Ancestors: []string{},
		PR: &PRView{
			Number: 7,
			State:  "OPEN",
			URL:    "https://github.com/org/repo/pull/7",
		},
		Commits: []CommitView{
			{SHA: "111", Subject: "first"},
		},
	}
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	for _, key := range []string{
		`"branch":"feat-a"`,
		`"trunk":"main"`,
		`"tracked":true`,
		`"parent":"main"`,
		`"children":["feat-b","feat-c"]`,
		`"pr":`,
		`"commits":`,
	} {
		if !strings.Contains(got, key) {
			t.Fatalf("expected %s in JSON, got %s", key, got)
		}
	}
}

func TestBranchViewOmitsEmptyOptionals(t *testing.T) {
	view := &BranchView{Branch: "x", Trunk: "main"}
	data, _ := json.Marshal(view)
	got := string(data)
	for _, banned := range []string{`"parent":`, `"parentSHA":`, `"tip":`, `"pr":`, `"commits":`, `"children":`, `"ancestors":`} {
		if strings.Contains(got, banned) {
			t.Fatalf("expected %s to be omitted, got %s", banned, got)
		}
	}
}
