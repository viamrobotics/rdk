package imu

import (
	"context"
	"github.com/edaniels/golog"
	"go.viam.com/rdk/data"
	"os"
	"time"
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

func newReadOrientationCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, logger golog.Logger) (data.Collector, error) {
	imu, err := ensureIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadOrientation(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readOrientation.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func newReadAccelerationCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, logger golog.Logger) (data.Collector, error) {
	imu, err := ensureIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAcceleration(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readAcceleration.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func newReadAngularVelocityCollector(resource interface{}, name string, interval time.Duration,
	params map[string]string, target *os.File, logger golog.Logger) (data.Collector, error) {
	imu, err := ensureIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAngularVelocity(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readAngularVelocity.String())
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, logger), nil
}

func ensureIMU(resource interface{}) (IMU, error) {
	imu, ok := resource.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return imu, nil
}
