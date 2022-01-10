// Package register registers all relevant GPSs and also subtype specific functions
package register

import (
	"go.viam.com/rdk/component/gps"

	// for GPSs.
	_ "go.viam.com/rdk/component/gps/fake"
	_ "go.viam.com/rdk/component/gps/merge"
	_ "go.viam.com/rdk/component/gps/nmea"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterResourceSubtype(gps.Subtype, registry.ResourceSubtype{
		Reconfigurable: gps.WrapWithReconfigurable,
	})
}
