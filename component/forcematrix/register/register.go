// Package register registers all relevant ForceMatrix's and
// also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/forcematrix"

	// register various implementations of ForceMatrix.
	_ "go.viam.com/rdk/component/forcematrix/fake"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixtraditional"
	_ "go.viam.com/rdk/component/forcematrix/vforcematrixwithmux"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(forcematrix.Subtype, registry.ResourceSubtype{
		Reconfigurable: forcematrix.WrapWithReconfigurable,
	})
}
