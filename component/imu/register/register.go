// Package register registers all relevant IMUs and also subtype specific functions
package register

import (
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"

	"go.viam.com/core/component/imu"
	_ "go.viam.com/core/component/imu/fake" // for imu
	_ "go.viam.com/core/component/imu/wit"  // for imu
)

func init() {
	registry.RegisterResourceSubtype(imu.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return imu.WrapWithReconfigurable(r)
		},
	})
}
