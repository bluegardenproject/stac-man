package git

import (
	"errors"
	"os"
)

// pathExists reports whether a filesystem path exists. Used by
// RebaseInProgress to detect .git/rebase-merge / .git/rebase-apply.
func pathExists(p string) (bool, error) {
	_, err := os.Stat(p)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
