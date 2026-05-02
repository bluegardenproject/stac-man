package gh

import (
	"context"
	"path/filepath"
	"testing"
	"time"
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

func TestChecksCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := LoadChecksCache(dir)
	if got, ok := c.Get(7); ok {
		t.Fatalf("expected miss on empty cache, got %+v", got)
	}
	c.Put(7, PRStatus{Number: 7, Checks: ChecksPass, Mergeable: MergeMergeable, State: PRStateOpen})
	if err := c.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// A separate Load should see the entry.
	got, ok := LoadChecksCache(dir).Get(7)
	if !ok {
		t.Fatalf("expected cache hit after save")
	}
	if got.Checks != ChecksPass || got.Mergeable != MergeMergeable {
		t.Fatalf("unexpected cached PRStatus: %+v", got)
	}
}

func TestChecksCacheRespectsTTL(t *testing.T) {
	dir := t.TempDir()
	c := LoadChecksCache(dir)
	c.Put(99, PRStatus{Number: 99, Checks: ChecksPass})
	// Hand-stomp the FetchedAt to simulate an expired entry without
	// sleeping the full TTL.
	entry := c.Entries["99"]
	entry.FetchedAt = time.Now().Add(-2 * checksCacheTTL)
	c.Entries["99"] = entry
	if _, ok := c.Get(99); ok {
		t.Fatalf("expected expired entry to be a cache miss")
	}
}

func TestInvalidateChecksCacheRemovesFile(t *testing.T) {
	dir := t.TempDir()
	c := LoadChecksCache(dir)
	c.Put(1, PRStatus{Number: 1})
	if err := c.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := InvalidateChecksCache(dir); err != nil {
		t.Fatalf("InvalidateChecksCache: %v", err)
	}
	// Subsequent invalidate is a no-op.
	if err := InvalidateChecksCache(dir); err != nil {
		t.Fatalf("second InvalidateChecksCache: %v", err)
	}
	// And subsequent load returns empty.
	if got, ok := LoadChecksCache(dir).Get(1); ok {
		t.Fatalf("expected cache empty after invalidate, got %+v", got)
	}
}

func TestChecksCachePathIsUnderGitDir(t *testing.T) {
	got := ChecksCachePath("/tmp/x/.git")
	want := filepath.Join("/tmp/x/.git", "stac-man", "checks-cache.json")
	if got != want {
		t.Fatalf("ChecksCachePath = %s, want %s", got, want)
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
