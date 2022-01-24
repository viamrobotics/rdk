// Package gps defines the interfaces of a GPS device which provides lat/long
// measurements.
package gps

import (
	"context"
	"sync"

	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
)

// SubtypeName is a constant that identifies the component resource subtype string "gps".
const SubtypeName = resource.SubtypeName("gps")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named GPS's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A GPS represents a GPS that can report lat/long measurements.
type GPS interface {
	sensor.Sensor
	ReadLocation(ctx context.Context) (*geo.Point, error) // The current latitude and longitude
	ReadAltitude(ctx context.Context) (float64, error)    // The current altitude in meters
	ReadSpeed(ctx context.Context) (float64, error)       // Current ground speed in kph
}

// A LocalGPS represents a GPS that can report accuracy, satellites and valid measurements.
type LocalGPS interface {
	GPS
	ReadAccuracy(ctx context.Context) (float64, float64, error) // Horizontal and vertical position error in meters
	ReadSatellites(ctx context.Context) (int, int, error)       // Number of satellites used for fix, and total in view
	ReadValid(ctx context.Context) (bool, error)                // Whether or not the GPS chip had a valid fix for the most recent dataset
}

var (
	_ = LocalGPS(&reconfigurableGPS{})
	_ = resource.Reconfigurable(&reconfigurableGPS{})
)

type reconfigurableGPS struct {
	mu     sync.RWMutex
	actual LocalGPS
}

func (r *reconfigurableGPS) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return utils.TryClose(ctx, r.actual)
}

func (r *reconfigurableGPS) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableGPS) ReadLocation(ctx context.Context) (*geo.Point, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadLocation(ctx)
}

func (r *reconfigurableGPS) ReadAltitude(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAltitude(ctx)
}

func (r *reconfigurableGPS) ReadSpeed(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSpeed(ctx)
}

func (r *reconfigurableGPS) ReadSatellites(ctx context.Context) (int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadSatellites(ctx)
}

func (r *reconfigurableGPS) ReadAccuracy(ctx context.Context) (float64, float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadAccuracy(ctx)
}

func (r *reconfigurableGPS) ReadValid(ctx context.Context) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.ReadValid(ctx)
}

func (r *reconfigurableGPS) Readings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx)
}

func (r *reconfigurableGPS) Reconfigure(ctx context.Context, newGPS resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newGPS.(*reconfigurableGPS)
	if !ok {
		return errors.Errorf("expected new GPS to be %T but got %T", r, newGPS)
	}
	if err := utils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular LocalGPS implementation to a reconfigurableGPS.
// If GPS is already a reconfigurableGPS, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	gps, ok := r.(LocalGPS)
	if !ok {
		return nil, errors.Errorf("expected resource to be GPS but got %T", r)
	}
	if reconfigurable, ok := gps.(*reconfigurableGPS); ok {
		return reconfigurable, nil
	}
	return &reconfigurableGPS{actual: gps}, nil
}
