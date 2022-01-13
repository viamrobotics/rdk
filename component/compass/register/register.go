// Package register registers all relevant Compasses and also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/compass"

	// for Compasses.
	_ "go.viam.com/rdk/component/compass/fake"
	_ "go.viam.com/rdk/component/compass/gy511"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(compass.Subtype, registry.ResourceSubtype{
		Reconfigurable: compass.WrapWithReconfigurable,
	})
}
