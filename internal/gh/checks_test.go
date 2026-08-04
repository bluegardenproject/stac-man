package gh

import (
	"context"
	"testing"
)

func TestRollupFromEntriesEmptyIsNone(t *testing.T) {
	if got := rollupFromEntries(nil); got != ChecksNone {
		t.Fatalf("rollupFromEntries(nil) = %s, want %s", got, ChecksNone)
	}
}

func TestRollupFromEntriesAllSuccessIsPass(t *testing.T) {
	got := rollupFromEntries([]checkEntry{
		{State: "COMPLETED", Status: "COMPLETED", Conclusion: "SUCCESS"},
		{State: "COMPLETED", Status: "COMPLETED", Conclusion: "SKIPPED"},
		{State: "COMPLETED", Status: "COMPLETED", Conclusion: "NEUTRAL"},
	})
	if got != ChecksPass {
		t.Fatalf("got %s, want %s", got, ChecksPass)
	}
}

func TestRollupFromEntriesAnyFailureWins(t *testing.T) {
	got := rollupFromEntries([]checkEntry{
		{Conclusion: "SUCCESS"},
		{Conclusion: "FAILURE"},
		{Conclusion: "SUCCESS"},
	})
	if got != ChecksFail {
		t.Fatalf("got %s, want %s", got, ChecksFail)
	}
}

func TestRollupFromEntriesPendingBeatsSuccess(t *testing.T) {
	got := rollupFromEntries([]checkEntry{
		{Conclusion: "SUCCESS"},
		{Status: "IN_PROGRESS"},
	})
	if got != ChecksPending {
		t.Fatalf("got %s, want %s", got, ChecksPending)
	}
}

func TestRollupFromEntriesFailingBeatsPending(t *testing.T) {
	got := rollupFromEntries([]checkEntry{
		{Status: "IN_PROGRESS"},
		{Conclusion: "FAILURE"},
	})
	if got != ChecksFail {
		t.Fatalf("got %s, want %s", got, ChecksFail)
	}
}

// TestRollupFromEntriesLegacyStatusOnly covers commit-status entries
// that only set `state` (e.g. travis-style external status) and have
// no conclusion field at all.
func TestRollupFromEntriesLegacyStatusOnly(t *testing.T) {
	got := rollupFromEntries([]checkEntry{
		{State: "SUCCESS"},
		{State: "SUCCESS"},
	})
	if got != ChecksPass {
		t.Fatalf("got %s, want %s", got, ChecksPass)
	}

	got = rollupFromEntries([]checkEntry{
		{State: "SUCCESS"},
		{State: "PENDING"},
	})
	if got != ChecksPending {
		t.Fatalf("got %s, want %s", got, ChecksPending)
	}

	got = rollupFromEntries([]checkEntry{
		{State: "FAILURE"},
		{State: "SUCCESS"},
	})
	if got != ChecksFail {
		t.Fatalf("got %s, want %s", got, ChecksFail)
	}
}

func TestParseMergeable(t *testing.T) {
	cases := []struct {
		in   string
		want Mergeability
	}{
		{"MERGEABLE", MergeMergeable},
		{"mergeable", MergeMergeable},
		{"CONFLICTING", MergeConflicting},
		{"UNKNOWN", MergeUnknown},
		{"", MergeUnknown},
		{"weird", MergeUnknown},
	}
	for _, tc := range cases {
		if got := parseMergeable(tc.in); got != tc.want {
			t.Errorf("parseMergeable(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestPRStatusForNumberParsesGhJSON(t *testing.T) {
	r := &fakeRunner{
		fallback: fakeResponse{
			stdout: `{
				"number": 42,
				"mergeable": "CONFLICTING",
				"state": "OPEN",
				"isDraft": false,
				"statusCheckRollup": [
					{"state":"COMPLETED","status":"COMPLETED","conclusion":"SUCCESS"},
					{"state":"COMPLETED","status":"COMPLETED","conclusion":"FAILURE"}
				]
			}`,
		},
	}
	c := NewWithRunner(r)
	got, err := c.PRStatusForNumber(context.Background(), 42)
	if err != nil {
		t.Fatalf("PRStatusForNumber: %v", err)
	}
	if got.Number != 42 {
		t.Fatalf("Number = %d, want 42", got.Number)
	}
	if got.Mergeable != MergeConflicting {
		t.Fatalf("Mergeable = %s, want CONFLICTING", got.Mergeable)
	}
	if got.Checks != ChecksFail {
		t.Fatalf("Checks = %s, want %s", got.Checks, ChecksFail)
	}
	if got.State != PRStateOpen {
		t.Fatalf("State = %s, want OPEN", got.State)
	}
	if got.IsDraft {
		t.Fatalf("IsDraft = true, want false")
	}
	// gh CLI is invoked with the four required json fields.
	args := r.calls[0]
	for _, want := range []string{"pr", "view", "42", "--json"} {
		if !contains(args, want) {
			t.Fatalf("expected %q in args, got %v", want, args)
		}
	}
	jsonArg := args[len(args)-1]
	for _, key := range []string{"mergeable", "statusCheckRollup", "state", "isDraft"} {
		if !containsSubstring(jsonArg, key) {
			t.Fatalf("expected json arg to request %q, got %q", key, jsonArg)
		}
	}
}

func containsSubstring(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
