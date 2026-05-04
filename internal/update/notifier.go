package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CheckInterval is how often the notifier will hit the GitHub
// releases API in the background. Anything more frequent risks
// rate-limiting unauthenticated users; anything less frequent and the
// nag goes stale.
const CheckInterval = 24 * time.Hour

// cachedCheck is the persisted JSON shape. Keep field names stable —
// older binaries on disk still need to be able to read this file
// after an update.
type cachedCheck struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// CachePath returns the file the notifier writes to. Honors
// $XDG_CACHE_HOME and falls back to ~/.cache/stac-man/. The cache
// directory is intentionally separate from the config directory so
// that 'sm config edit' shows only user-managed YAML.
func CachePath() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "stac-man", "last-update-check.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "stac-man", "last-update-check.json"), nil
}

// Notifier coordinates a background refresh of the cached "latest
// release" tag and a synchronous read of whatever's currently
// cached. It is intentionally fire-and-forget: callers spawn the
// background fetch via Refresh() and check the cache later via
// CachedLatest() — there is no waiting on the goroutine. If the
// fetch is still running when the process exits, the goroutine is
// killed and the next run will retry. This keeps every command
// snappy regardless of GitHub latency.
type Notifier struct {
	once sync.Once
}

// Refresh kicks off a background fetch when the cache is older than
// CheckInterval (or missing). Safe to call from PersistentPreRunE on
// every invocation: only the first call per process actually does
// anything, and the goroutine has its own short network timeout.
func (n *Notifier) Refresh(ctx context.Context) {
	n.once.Do(func() {
		path, err := CachePath()
		if err != nil {
			return
		}
		if !shouldRefresh(path) {
			return
		}
		go func() {
			fetchCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			rel, err := LatestRelease(fetchCtx)
			if err != nil {
				return
			}
			_ = writeCache(path, cachedCheck{
				CheckedAt: time.Now().UTC(),
				Latest:    rel.TagName,
			})
		}()
	})
}

// CachedLatest returns the latest release tag we know about and a
// bool indicating whether the cache had any entry. Safe to call
// without ever calling Refresh() — it just returns ("", false) when
// the cache is empty.
func CachedLatest() (string, bool) {
	path, err := CachePath()
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var c cachedCheck
	if err := json.Unmarshal(data, &c); err != nil {
		return "", false
	}
	if c.Latest == "" {
		return "", false
	}
	return c.Latest, true
}

func shouldRefresh(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true
		}
		return true
	}
	var c cachedCheck
	if err := json.Unmarshal(data, &c); err != nil {
		return true
	}
	return time.Since(c.CheckedAt) >= CheckInterval
}

func writeCache(path string, c cachedCheck) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cache.*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}
