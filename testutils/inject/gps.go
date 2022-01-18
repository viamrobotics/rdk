package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/component/sensor"
)

// GPS is an injected GPS.
type GPS struct {
	gps.LocalGPS
	LocationFunc   func(ctx context.Context) (*geo.Point, error)
	AltitudeFunc   func(ctx context.Context) (float64, error)
	SpeedFunc      func(ctx context.Context) (float64, error)
	SatellitesFunc func(ctx context.Context) (int, int, error)
	AccuracyFunc   func(ctx context.Context) (float64, float64, error)
	ValidFunc      func(ctx context.Context) (bool, error)
	ReadingsFunc   func(ctx context.Context) ([]interface{}, error)
	DescFunc       func() sensor.Description
	CloseFunc      func(ctx context.Context) error
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

// Satellites calls the injected Satellites or the real version.
func (i *GPS) Satellites(ctx context.Context) (int, int, error) {
	if i.SatellitesFunc == nil {
		return i.LocalGPS.Satellites(ctx)
	}
	return i.SatellitesFunc(ctx)
}

// Accuracy calls the injected Accuracy or the real version.
func (i *GPS) Accuracy(ctx context.Context) (float64, float64, error) {
	if i.AccuracyFunc == nil {
		return i.LocalGPS.Accuracy(ctx)
	}
	return i.AccuracyFunc(ctx)
}

// Valid calls the injected Valid or the real version.
func (i *GPS) Valid(ctx context.Context) (bool, error) {
	if i.ValidFunc == nil {
		return i.LocalGPS.Valid(ctx)
	}
	return i.ValidFunc(ctx)
}

// Readings calls the injected Readings or the real version.
func (i *GPS) Readings(ctx context.Context) ([]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.LocalGPS.Readings(ctx)
	}
	return i.ReadingsFunc(ctx)
}

// Desc returns that this is an GPS.
func (i *GPS) Desc() sensor.Description {
	if i.DescFunc == nil {
		return i.LocalGPS.Desc()
	}
	return i.DescFunc()
}

// Close calls the injected Close or the real version.
func (i *GPS) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.LocalGPS)
	}
	return i.CloseFunc(ctx)
}
