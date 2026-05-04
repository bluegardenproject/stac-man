package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withFakeCache redirects CachePath() at a temp dir for the test,
// then deletes the global state so tests don't bleed into each other.
func withFakeCache(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)
	return dir
}

func TestCachePath_HonorsXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/tmp/xdg-cache-fake")

	p, err := CachePath()
	if err != nil {
		t.Fatal(err)
	}
	if want := "/tmp/xdg-cache-fake/stac-man/last-update-check.json"; p != want {
		t.Errorf("CachePath = %q; want %q", p, want)
	}
}

func TestCachedLatest_ReturnsFalseWhenMissing(t *testing.T) {
	withFakeCache(t)

	if v, ok := CachedLatest(); ok {
		t.Errorf("CachedLatest = %q, true; want \"\", false on missing cache", v)
	}
}

func TestCachedLatest_ReturnsValueWhenPresent(t *testing.T) {
	xdg := withFakeCache(t)
	dir := filepath.Join(xdg, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(cachedCheck{
		CheckedAt: time.Now().UTC(),
		Latest:    "v9.9.9",
	})
	if err := os.WriteFile(filepath.Join(dir, "last-update-check.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}

	v, ok := CachedLatest()
	if !ok {
		t.Fatal("CachedLatest reported missing on a populated cache")
	}
	if v != "v9.9.9" {
		t.Errorf("CachedLatest = %q; want v9.9.9", v)
	}
}

func TestCachedLatest_IgnoresMalformedCache(t *testing.T) {
	xdg := withFakeCache(t)
	dir := filepath.Join(xdg, "stac-man")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "last-update-check.json"), []byte("not-json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, ok := CachedLatest(); ok {
		t.Error("CachedLatest accepted malformed JSON; expected (\"\", false)")
	}
}

func TestShouldRefresh(t *testing.T) {
	xdg := withFakeCache(t)
	path := filepath.Join(xdg, "stac-man", "last-update-check.json")

	if !shouldRefresh(path) {
		t.Error("shouldRefresh on missing file = false; want true")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}

	// Fresh cache — should NOT refresh.
	body, _ := json.Marshal(cachedCheck{
		CheckedAt: time.Now().UTC().Add(-1 * time.Hour),
		Latest:    "v1.0.0",
	})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if shouldRefresh(path) {
		t.Error("shouldRefresh on 1h-old cache = true; want false")
	}

	// Stale cache — should refresh.
	body, _ = json.Marshal(cachedCheck{
		CheckedAt: time.Now().UTC().Add(-CheckInterval - time.Minute),
		Latest:    "v1.0.0",
	})
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if !shouldRefresh(path) {
		t.Error("shouldRefresh on stale cache = false; want true")
	}
}

func TestWriteCache_AtomicAndReReadable(t *testing.T) {
	xdg := withFakeCache(t)
	path := filepath.Join(xdg, "stac-man", "last-update-check.json")

	now := time.Now().UTC().Truncate(time.Second)
	if err := writeCache(path, cachedCheck{CheckedAt: now, Latest: "v0.5.0"}); err != nil {
		t.Fatalf("writeCache: %v", err)
	}

	v, ok := CachedLatest()
	if !ok || v != "v0.5.0" {
		t.Fatalf("after writeCache, CachedLatest = (%q, %v); want (v0.5.0, true)", v, ok)
	}

	// No tempfile leftovers.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "last-update-check.json" {
			t.Errorf("unexpected leftover in cache dir: %s", e.Name())
		}
	}
}
