package imu

import (
	"context"

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

func newReadOrientationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadOrientation(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readOrientation.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newReadAccelerationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAcceleration(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readAcceleration.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newReadAngularVelocityCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	imu, err := assertIMU(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := imu.ReadAngularVelocity(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readAngularVelocity.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertIMU(resource interface{}) (IMU, error) {
	imu, ok := resource.(IMU)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return imu, nil
}
