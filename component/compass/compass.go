// Package compass defines the interfaces of a Compass and a Relative Compass which
// provide yaw measurements.
package compass

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/sensor"
	"go.viam.com/rdk/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "compass".
const SubtypeName = resource.SubtypeName("compass")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named Compass's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// A Compass represents a Compass that can report yaw measurements. It can also
// be requested to calibrate itself.
type Compass interface {
	sensor.Sensor
	Heading(ctx context.Context) (float64, error)
	StartCalibration(ctx context.Context) error
	StopCalibration(ctx context.Context) error
}

// A Markable denotes a RelativeCompass, a Compass that has no reference point (like a magnetic north)
// and as such must be asked to "mark" its current position as zero degrees.
type Markable interface {
	Mark(ctx context.Context) error
}

var (
	_ = Compass(&reconfigurableCompass{})
	_ = resource.Reconfigurable(&reconfigurableCompass{})
)

type reconfigurableCompass struct {
	mu     sync.RWMutex
	actual Compass
}

func (r *reconfigurableCompass) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableCompass) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableCompass) Heading(ctx context.Context) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Heading(ctx)
}

func (r *reconfigurableCompass) StartCalibration(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.StartCalibration(ctx)
}

func (r *reconfigurableCompass) StopCalibration(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.StopCalibration(ctx)
}

func (r *reconfigurableCompass) Mark(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rActual, ok := r.actual.(Markable)
	if !ok {
		return errors.New("compass is not Markable")
	}
	return rActual.Mark(ctx)
}

func (r *reconfigurableCompass) Readings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx)
}

func (r *reconfigurableCompass) Desc() sensor.Description {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Desc()
}

func (r *reconfigurableCompass) Reconfigure(ctx context.Context, newCompass resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newCompass.(*reconfigurableCompass)
	if !ok {
		return errors.Errorf("expected new Compass to be %T but got %T", r, newCompass)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Compass implementation to a reconfigurableCompass.
// If Compass is already a reconfigurableCompass, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	compass, ok := r.(Compass)
	if !ok {
		return nil, errors.Errorf("expected resource to be Compass but got %T", r)
	}
	if reconfigurable, ok := compass.(*reconfigurableCompass); ok {
		return reconfigurable, nil
	}
	return &reconfigurableCompass{actual: compass}, nil
}

// MedianHeading returns the median of successive headings from the given compass.
func MedianHeading(ctx context.Context, device Compass) (float64, error) {
	var headings []float64
	numReadings := 5
	for i := 0; i < numReadings; i++ {
		heading, err := device.Heading(ctx)
		if err != nil {
			return 0, err
		}
		headings = append(headings, heading)
	}
	return utils.Median(headings...), nil
}
