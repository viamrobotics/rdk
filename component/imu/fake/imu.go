// Package fake implements a fake IMU.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterComponent(imu.Subtype, "fake", registry.Component{
		Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			if config.Attributes.Bool("fail_new", false) {
				return nil, errors.New("whoops")
			}
			return NewIMU(config)
		},
	})
}

// NewIMU returns a new fake IMU.
func NewIMU(cfg config.Component) (imu.IMU, error) {
	name := cfg.Name
	return &IMU{
		Name:            name,
		angularVelocity: spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
		orientation:     spatialmath.EulerAngles{Roll: utils.DegToRad(1), Pitch: utils.DegToRad(2), Yaw: utils.DegToRad(3)},
		acceleration:    r3.Vector{X: 1, Y: 2, Z: 3},
		magnetometer:    r3.Vector{X: 1, Y: 2, Z: 3},
	}, nil
}

// IMU is a fake IMU device that always returns the set angular velocity and orientation.
type IMU struct {
	Name            string
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles
	acceleration    r3.Vector
	magnetometer    r3.Vector

	mu sync.Mutex
}

// ReadAngularVelocity always returns the set value.
func (i *IMU) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.angularVelocity, nil
}

// ReadOrientation always returns the set value.
func (i *IMU) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return &i.orientation, nil
}

// ReadAcceleration always returns the set value.
func (i *IMU) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.acceleration, nil
}

// ReadMagnetometer always returns the set value.
func (i *IMU) ReadMagnetometer(ctx context.Context) (r3.Vector, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.magnetometer, nil
}
