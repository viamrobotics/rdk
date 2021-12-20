package fake

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"go.viam.com/core/sensor"
	"go.viam.com/core/spatialmath"

	"go.viam.com/core/component/imu"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"

	"github.com/edaniels/golog"
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
		Latitude:        0,
		Longitude:       0,
		angularVelocity: spatialmath.AngularVelocity{X: 1, Y: 2, Z: 3},
		orientation:     spatialmath.EulerAngles{Roll: 1, Pitch: 2, Yaw: 3},
	}, nil
}

// IMU is a fake IMU device that always returns the set angular velocity and orientation.
type IMU struct {
	Name            string
	Latitude        float64
	Longitude       float64
	angularVelocity spatialmath.AngularVelocity
	orientation     spatialmath.EulerAngles

	mu sync.Mutex
}

// AngularVelocity always returns the set value.
func (i *IMU) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.angularVelocity, nil
}

// Orientation always returns the set value.
func (i *IMU) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return &i.orientation, nil
}

// Desc returns that this is an IMU.
func (i *IMU) Desc() sensor.Description {
	return sensor.Description{sensor.Type(imu.SubtypeName), ""}
}

// Readings always returns the set values.
func (i *IMU) Readings(ctx context.Context) ([]interface{}, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return []interface{}{i.Latitude, i.Longitude}, nil
}
