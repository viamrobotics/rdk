package imu

import (
	"context"
	"os"
	"time"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/data"
)

type method int64

const (
	readAngularVelocity method = iota
	readOrientation
	readAcceleration
)

func (m method) String() string {
	switch m {
	case readAngularVelocity:
		return "ReadAngularVelocity"
	case readOrientation:
		return "ReadOrientation"
	case readAcceleration:
		return "ReadAcceleration"
	}
	return "Unknown"
}

type readAngularVelocityCapturer struct {
	imu  IMU
	name string
}

type readOrientationCapturer struct {
	imu  IMU
	name string
}

type readAccelerationCapturer struct {
	imu  IMU
	name string
}

// Capture returns an *any.Any containing the response of a single ReadAngularVelocity call on the backing client.
func (c readAngularVelocityCapturer) Capture(ctx context.Context, _ map[string]string) (interface{}, error) {
	v, err := c.imu.ReadAngularVelocity(ctx)
	if err != nil {
		return nil, data.FailedToReadErr(c.name, readAngularVelocity.String())
	}
	return v, nil
}

// Capture returns an *any.Any containing the response of a single ReadOrientation call on the backing client.
func (c readOrientationCapturer) Capture(ctx context.Context, _ map[string]string) (interface{}, error) {
	v, err := c.imu.ReadOrientation(ctx)
	if err != nil {
		return nil, data.FailedToReadErr(c.name, readOrientation.String())
	}
	return v, nil
}

// Capture returns an *any.Any containing the response of a single ReadAcceleration call on the backing client.
func (c readAccelerationCapturer) Capture(ctx context.Context, _ map[string]string) (interface{}, error) {
	v, err := c.imu.ReadAcceleration(ctx)
	if err != nil {
		return nil, data.FailedToReadErr(c.name, readAcceleration.String())
	}
	return v, nil
}

func newReadAngularVelocityCollector(imu interface{}, name string, interval time.Duration, params map[string]string, target *os.File,
	logger golog.Logger) (data.Collector, error) {
	validIMU, ok := imu.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	c := readAngularVelocityCapturer{imu: validIMU, name: name}
	return data.NewCollector(c, interval, params, target, logger), nil
}

func newReadAccelerationCollector(imu interface{}, name string, interval time.Duration, params map[string]string, target *os.File,
	logger golog.Logger) (data.Collector, error) {
	validIMU, ok := imu.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	c := readAccelerationCapturer{imu: validIMU, name: name}
	return data.NewCollector(c, interval, params, target, logger), nil
}

func newReadOrientationCollector(imu interface{}, name string, interval time.Duration, params map[string]string, target *os.File,
	logger golog.Logger) (data.Collector, error) {
	validIMU, ok := imu.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	c := readOrientationCapturer{imu: validIMU, name: name}
	return data.NewCollector(c, interval, params, target, logger), nil
}
