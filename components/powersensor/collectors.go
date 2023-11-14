package powersensor

import (
	"context"
	"errors"

	v1 "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/powersensor/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
)

type method int64

const (
	voltage method = iota
	current
	power
	readings
)

func (m method) String() string {
	switch m {
	case voltage:
		return "Voltage"
	case current:
		return "Current"
	case power:
		return "Power"
	case readings:
		return "Readings"
	}
	return "Unknown"
}

func assertPowerSensor(resource interface{}) (PowerSensor, error) {
	ps, ok := resource.(PowerSensor)
	if !ok {
		return nil, data.InvalidInterfaceErr(API)
	}
	return ps, nil
}

// NewVoltageCollector returns a collector to register a voltage method. If one is already registered
// with the same MethodMetadata it will panic.
func NewVoltageCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		volts, isAc, err := ps.Voltage(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, voltage.String(), err)
		}
		return pb.GetVoltageResponse{
			Volts: volts,
			IsAc:  isAc,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// NewCurrentCollector returns a collector to register a current method. If one is already registered
// with the same MethodMetadata it will panic.
func NewCurrentCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		curr, isAc, err := ps.Current(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, current.String(), err)
		}
		return pb.GetCurrentResponse{
			Amperes: curr,
			IsAc:    isAc,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// NewPowerCollector returns a collector to register a power method. If one is already registered
// with the same MethodMetadata it will panic.
func NewPowerCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (interface{}, error) {
		pwr, err := ps.Power(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, power.String(), err)
		}
		return pb.GetPowerResponse{
			Watts: pwr,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// NewReadingsCollector returns a collector to register a readings method. If one is already registered
// with the same MethodMetadata it will panic.
func NewReadingsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (interface{}, error) {
		values, err := ps.Readings(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return nil, err
			}
			return nil, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}
		readings := make(map[string]*structpb.Value)
		for name, value := range values {
			val, err := structpb.NewValue(value)
			if err != nil {
				return nil, err
			}
			readings[name] = val
		}
		return v1.GetReadingsResponse{
			Readings: readings,
		}, nil
	})
	return data.NewCollector(cFunc, params)
}
