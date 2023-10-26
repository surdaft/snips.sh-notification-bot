package shoutrrr

import (
	"strings"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/sagikazarmark/slog-shim"
)

var DefaultClient *router.ServiceRouter

func New(shoutrrrURIs []string) *router.ServiceRouter {
	if DefaultClient != nil {
		slog.Info(strings.Join(shoutrrrURIs, ", "))
		s, err := shoutrrr.CreateSender(shoutrrrURIs...)
		if err != nil {
			panic(err)
		}

		DefaultClient = s
	}

	return DefaultClient
}
