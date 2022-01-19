// Package imu defines the interface of an IMU providing angular velocity, roll,
// pitch, and yaw measurements.
package imu

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	viamutils "go.viam.com/utils"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/spatialmath"
)

// SubtypeName is a constant that identifies the component resource subtype string "imu".
const SubtypeName = resource.SubtypeName("imu")

// Subtype is a constant that identifies the component resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeComponent,
	SubtypeName,
)

// Named is a helper for getting the named IMU's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// An IMU represents a sensor that can report AngularVelocity and Orientation measurements.
type IMU interface {
	sensor.Sensor
	AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error)
	Orientation(ctx context.Context) (spatialmath.Orientation, error)
}

var (
	_ = IMU(&reconfigurableIMU{})
	_ = resource.Reconfigurable(&reconfigurableIMU{})
)

type reconfigurableIMU struct {
	mu     sync.RWMutex
	actual IMU
}

func (r *reconfigurableIMU) Close(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return viamutils.TryClose(ctx, r.actual)
}

func (r *reconfigurableIMU) ProxyFor() interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual
}

func (r *reconfigurableIMU) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.AngularVelocity(ctx)
}

func (r *reconfigurableIMU) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Orientation(ctx)
}

func (r *reconfigurableIMU) Readings(ctx context.Context) ([]interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Readings(ctx)
}

func (r *reconfigurableIMU) Desc(ctx context.Context) (sensor.Description, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.actual.Desc(ctx)
}

func (r *reconfigurableIMU) Reconfigure(ctx context.Context, newIMU resource.Reconfigurable) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	actual, ok := newIMU.(*reconfigurableIMU)
	if !ok {
		return errors.Errorf("expected new IMU to be %T but got %T", r, newIMU)
	}
	if err := viamutils.TryClose(ctx, r.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	r.actual = actual.actual
	return nil
}

// WrapWithReconfigurable converts a regular IMU implementation to a reconfigurableIMU.
// If imu is already a reconfigurableIMU, then nothing is done.
func WrapWithReconfigurable(r interface{}) (resource.Reconfigurable, error) {
	imu, ok := r.(IMU)
	if !ok {
		return nil, errors.Errorf("expected resource to be IMU but got %T", r)
	}
	if reconfigurable, ok := imu.(*reconfigurableIMU); ok {
		return reconfigurable, nil
	}
	return &reconfigurableIMU{actual: imu}, nil
}
