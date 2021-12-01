package lidar

import (
	"context"
	"sync"

	"github.com/go-errors/errors"
	"github.com/golang/geo/r2"
	"go.viam.com/core/resource"
	"go.viam.com/core/rlog"
	viamutils "go.viam.com/utils"
)

// SubtypeName is a constant that identifies the component resource subtype string "servo"
const SubtypeName = resource.SubtypeName("lidar")

// Subtype is a constant that identifies the component resource subtype
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceCore,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// A Lidar represents a LiDAR that can scan an area and return metadata
// about itself. Currently it is only suitable for 2D measurements
// but can be easily expanded to 3D.
type Lidar interface {
	// Info returns metadata about the lidar.
	Info(ctx context.Context) (map[string]interface{}, error)

	// Start starts the lidar to prepare for scanning and should
	// only be called once during its lifetime unless stop is called.
	Start(ctx context.Context) error

	// Stop stops the lidar and prevents future scans; it can be called at any time.
	Stop(ctx context.Context) error

	// Scan returns measurements oriented from the pose at the moment
	// the method was called. Multiple measurements at the same angle are
	// permitted to be returned.
	Scan(ctx context.Context, options ScanOptions) (Measurements, error)

	// Range returns the maximum distance in millimeters that the lidar can
	// reliably report measurements at.
	Range(ctx context.Context) (float64, error)

	// Bounds returns a two-dimensional bounding box of where measurements will
	// fall in represented in millimeters.
	Bounds(ctx context.Context) (r2.Point, error)

	// AngularResolution reports the minimum distance in degrees that the lidar
	// can produce measurements for.
	AngularResolution(ctx context.Context) (float64, error)
}

// Named is a helper for getting the named Arm's typed resource name
func Named(name string) resource.Name {
	return resource.NewFromSubtype(Subtype, name)
}

var (
	_ = Lidar(&reconfigurableLidar{})
	_ = resource.Reconfigurable(&reconfigurableLidar{})
)

type reconfigurableLidar struct {
	mu     sync.RWMutex
	actual Lidar
}

func (_lidar *reconfigurableLidar) ProxyFor() interface{} {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual
}

func (_lidar *reconfigurableLidar) Info(ctx context.Context) (map[string]interface{}, error) {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Info(ctx)
}

func (_lidar *reconfigurableLidar) Start(ctx context.Context) error {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Start(ctx)
}

func (_lidar *reconfigurableLidar) Stop(ctx context.Context) error {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Stop(ctx)
}

func (_lidar *reconfigurableLidar) Scan(ctx context.Context, options ScanOptions) (Measurements, error) {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Scan(ctx, options)
}

func (_lidar *reconfigurableLidar) Range(ctx context.Context) (float64, error) {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Range(ctx)
}

func (_lidar *reconfigurableLidar) Bounds(ctx context.Context) (r2.Point, error) {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.Bounds(ctx)
}

func (_lidar *reconfigurableLidar) AngularResolution(ctx context.Context) (float64, error) {
	_lidar.mu.RLock()
	defer _lidar.mu.Unlock()
	return _lidar.actual.AngularResolution(ctx)
}

func (_lidar *reconfigurableLidar) Close() error {
	_lidar.mu.RLock()
	defer _lidar.mu.RUnlock()
	return viamutils.TryClose(_lidar.actual)
}

func (r *reconfigurableLidar) Reconfigure(newLidar resource.Reconfigurable) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	actual, ok := newLidar.(*reconfigurableLidar)
	if !ok {
		return errors.Errorf("expected new arm to be %T but got %T", r, newLidar)
	}
	if err := viamutils.TryClose(r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular Lidar implementation to a reconfigurableLidar.
// If servo is already a reconfigurableLidar, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	servo, ok := r.(Lidar)
	if !ok {
		return nil, errors.Errorf("expected resource to be Lidar but got %T", r)
	}
	if reconfigurable, ok := servo.(*reconfigurableLidar); ok {
		return reconfigurable, nil
	}
	return &reconfigurableLidar{actual: servo}, nil
}

// Type identifies the type of lidar. These are typically registered in
// via RegisterType.
type Type string

// Some builtin device types.
const (
	TypeUnknown = "unknown"
	TypeFake    = "fake"
)

// ScanOptions modify how scan results will be produced and are subject
// to interpretation by the lidar implementation.
type ScanOptions struct {
	// Count determines how many scans to perform.
	Count int

	// NoFilter hints to the implementation to give as raw results as possible.
	// Normally an implementation may do some extra work to eliminate false
	// positives but this can be expensive and undesired.
	NoFilter bool
}
