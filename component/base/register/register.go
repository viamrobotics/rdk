// Package register registers all relevant bases and also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/registry"

	// register four-wheel and generic wheeled bases
	_ "go.viam.com/rdk/component/base/wheeled"
)

func init() {
	registry.RegisterResourceSubtype(base.Subtype, registry.ResourceSubtype{
		Reconfigurable: base.WrapWithReconfigurable,
	})

}
