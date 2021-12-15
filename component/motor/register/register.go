// Package register registers all relevant motors and also subtype specific functions
package register

import (
	"go.viam.com/core/component/motor"
	"go.viam.com/core/registry"
	"go.viam.com/core/resource"

	// all motor implementations should be imported here for
	// registration availability
	_ "go.viam.com/core/component/motor/fake"        // fake motor
	_ "go.viam.com/core/component/motor/gpio"        // pi motor
	_ "go.viam.com/core/component/motor/gpiostepper" // pi stepper motor
	_ "go.viam.com/core/component/motor/tmcstepper"  // tmc stepper motor
)

func init() {
	registry.RegisterResourceSubtype(motor.Subtype, registry.ResourceSubtype{
		Reconfigurable: func(r interface{}) (resource.Reconfigurable, error) {
			return motor.WrapWithReconfigurable(r)
		},
	})
}
