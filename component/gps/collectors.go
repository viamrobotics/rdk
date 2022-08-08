package gps

import (
	"context"

	"go.viam.com/rdk/data"
)

type method int64

const (
	readLocation method = iota
	readAltitude method = iota
	readSpeed    method = iota
)

func (m method) String() string {
	switch m {
	case readLocation:
		return "ReadLocation"
	case readAltitude:
		return "ReadAltitude"
	case readSpeed:
		return "ReadSpeed"
	}
	return "Unknown"
}

// TODO: add tests for this file.

func newReadLocationCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gps, err := assertGPS(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gps.ReadLocation(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readLocation.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newReadAltitudeCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gps, err := assertGPS(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gps.ReadAltitude(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readAltitude.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func newReadSpeedCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	gps, err := assertGPS(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, _ map[string]string) (interface{}, error) {
		v, err := gps.ReadSpeed(ctx)
		if err != nil {
			return nil, data.FailedToReadErr(params.ComponentName, readSpeed.String(), err)
		}
		return v, nil
	})
	return data.NewCollector(cFunc, params)
}

func assertGPS(resource interface{}) (GPS, error) {
	gps, ok := resource.(GPS)
	if !ok {
		return nil, data.InvalidInterfaceErr(SubtypeName)
	}
	return gps, nil
}
