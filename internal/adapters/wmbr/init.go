package wmbr

import "github.com/jnewton/tapedeck/pkg/tapedeck"

func init() {
	tapedeck.RegisterAdapter("WMBR", func() tapedeck.Adapter {
		return New()
	})
}
