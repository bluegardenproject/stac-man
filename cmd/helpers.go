package cmd

import (
	"github.com/philipptpunkt/stac-man/internal/service"
)

// newService returns a Service rooted at the current working
// directory. Subcommands call this in their RunE.
func newService() *service.Service {
	return service.New("")
}
