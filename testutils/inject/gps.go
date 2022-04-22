package inject

import (
	"context"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/gps"
	rdkutils "go.viam.com/rdk/utils"
)

// GPS is an injected GPS.
type GPS struct {
	gps.LocalGPS
	DoFunc             func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ReadLocationFunc   func(ctx context.Context) (*geo.Point, error)
	ReadAltitudeFunc   func(ctx context.Context) (float64, error)
	ReadSpeedFunc      func(ctx context.Context) (float64, error)
	ReadSatellitesFunc func(ctx context.Context) (int, int, error)
	ReadAccuracyFunc   func(ctx context.Context) (float64, float64, error)
	ReadValidFunc      func(ctx context.Context) (bool, error)
	CloseFunc          func(ctx context.Context) error
}

// ReadLocation calls the injected ReadLocation or the real version.
func (i *GPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	if i.ReadLocationFunc == nil {
		return i.LocalGPS.ReadLocation(ctx)
	}
	return i.ReadLocationFunc(ctx)
}

// ReadAltitude calls the injected ReadAltitude or the real version.
func (i *GPS) ReadAltitude(ctx context.Context) (float64, error) {
	if i.ReadAltitudeFunc == nil {
		return i.LocalGPS.ReadAltitude(ctx)
	}
	return i.ReadAltitudeFunc(ctx)
}

// ReadSpeed calls the injected ReadSpeed or the real version.
func (i *GPS) ReadSpeed(ctx context.Context) (float64, error) {
	if i.ReadSpeedFunc == nil {
		return i.LocalGPS.ReadSpeed(ctx)
	}
	return i.ReadSpeedFunc(ctx)
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

// Close calls the injected Close or the real version.
func (i *GPS) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		return utils.TryClose(ctx, i.LocalGPS)
	}
	return i.CloseFunc(ctx)
}

// Do calls the injected Do or the real version.
func (i *GPS) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		if doer, ok := i.LocalGPS.(generic.Generic); ok {
			return doer.Do(ctx, cmd)
		}
		return nil, rdkutils.NewUnimplementedInterfaceError("Generic", i.LocalGPS)
	}
	return i.DoFunc(ctx, cmd)
}
