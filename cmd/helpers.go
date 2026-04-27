package cmd

import (
	"github.com/philipptpunkt/stac-man/internal/service"
)

// newService returns a Service rooted at the current working
// directory. Subcommands call this in their RunE.
func newService() *service.Service {
	return service.New("")
}

// short returns the 7-char prefix of a SHA. cmd-layer-only utility.
func short(sha string) string {
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}
