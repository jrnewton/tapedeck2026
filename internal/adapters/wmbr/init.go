package wmbr

import "local/tapedeck/pkg/tapedeck"

func init() {
	tapedeck.RegisterAdapter("WMBR", func() tapedeck.Adapter {
		return New()
	})
}
