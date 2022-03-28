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

func newReadOrientationCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, bufferSize int, logger golog.Logger) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadOrientation(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readOrientation.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, bufferSize, logger), nil
}

func newReadAccelerationCollector(resource interface{}, name string, interval time.Duration, params map[string]string,
	target *os.File, queueSize int, bufferSize int, logger golog.Logger) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAcceleration(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readAcceleration.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, bufferSize, logger), nil
}

func newReadAngularVelocityCollector(resource interface{}, name string, interval time.Duration,
	params map[string]string, target *os.File, queueSize int, bufferSize int, logger golog.Logger) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAngularVelocity(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(name, readAngularVelocity.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, interval, params, target, queueSize, bufferSize, logger), nil
}

func assertIMU(resource interface{}) (IMU, error) {
	imu, ok := resource.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return imu, nil
}
