package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gps"
)

// GPS is an injected GPS.
type GPS struct {
	gps.LocalGPS
	LocationFunc       func(ctx context.Context) (*geo.Point, error)
	AltitudeFunc       func(ctx context.Context) (float64, error)
	SpeedFunc          func(ctx context.Context) (float64, error)
	ReadSatellitesFunc func(ctx context.Context) (int, int, error)
	ReadAccuracyFunc   func(ctx context.Context) (float64, float64, error)
	ReadValidFunc      func(ctx context.Context) (bool, error)
	ReadingsFunc       func(ctx context.Context) ([]interface{}, error)
	CloseFunc          func(ctx context.Context) error
}

// Location calls the injected Location or the real version.
func (i *GPS) Location(ctx context.Context) (*geo.Point, error) {
	if i.LocationFunc == nil {
		return i.LocalGPS.Location(ctx)
	}
	return i.LocationFunc(ctx)
}

// Altitude calls the injected Altitude or the real version.
func (i *GPS) Altitude(ctx context.Context) (float64, error) {
	if i.AltitudeFunc == nil {
		return i.LocalGPS.Altitude(ctx)
	}
	return i.AltitudeFunc(ctx)
}

// Speed calls the injected Speed or the real version.
func (i *GPS) Speed(ctx context.Context) (float64, error) {
	if i.SpeedFunc == nil {
		return i.LocalGPS.Speed(ctx)
	}
	return i.SpeedFunc(ctx)
}

// ReadSatellites calls the injected ReadSatellites or the real version.
func (i *GPS) ReadSatellites(ctx context.Context) (int, int, error) {
	if i.ReadSatellitesFunc == nil {
		return i.LocalGPS.ReadSatellites(ctx)
	}
	return i.ReadSatellitesFunc(ctx)
}

// ReadAccuracy calls the injected ReadAccuracy or the real version.
func (i *GPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	if i.ReadAccuracyFunc == nil {
		return i.LocalGPS.ReadAccuracy(ctx)
	}
	return i.ReadAccuracyFunc(ctx)
}

// ReadValid calls the injected ReadValid or the real version.
func (i *GPS) ReadValid(ctx context.Context) (bool, error) {
	if i.ReadValidFunc == nil {
		return i.LocalGPS.ReadValid(ctx)
	}
	return i.ReadValidFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (i *GPS) Readings(ctx context.Context) ([]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.LocalGPS.Readings(ctx)
	}
	return i.ReadingsFunc(ctx)
}

// Close calls the injected Close or the real version.
func (i *GPS) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.LocalGPS)
	}
	return i.CloseFunc(ctx)
}
