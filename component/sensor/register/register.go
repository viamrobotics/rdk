// Package register registers all relevant Sensors and also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/sensor"

	// for Sensors.
	_ "go.viam.com/rdk/component/sensor/fake"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(sensor.Subtype, registry.ResourceSubtype{
		Reconfigurable: sensor.WrapWithReconfigurable,
	})
}
