package powersensor

import (
	"context"
	"errors"

	pb "go.viam.com/api/component/powersensor/v1"
	uprotoutils "go.viam.com/utils/protoutils"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/data"
	"go.viam.com/rdk/protoutils"
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

//nolint: dupl
// newVoltageCollector returns a collector to register a voltage method. If one is already registered
// with the same MethodMetadata it will panic.
func newVoltageCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		var res data.CaptureResult
		volts, isAc, err := ps.Voltage(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, voltage.String(), err)
		}

		pbReading, err := uprotoutils.StructToStructPbIgnoreOmitEmpty(pb.GetVoltageResponse{
			Volts: volts,
			IsAc:  isAc,
		})
		if err != nil {
			return res, err
		}
		return data.CaptureResult{
			Type:        data.CaptureTypeTabular,
			TabularData: data.TabularData{Payload: pbReading},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

//nolint: dupl
// newCurrentCollector returns a collector to register a current method. If one is already registered
// with the same MethodMetadata it will panic.
func newCurrentCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		var res data.CaptureResult
		curr, isAc, err := ps.Current(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, current.String(), err)
		}
		pbReading, err := uprotoutils.StructToStructPbIgnoreOmitEmpty(pb.GetCurrentResponse{
			Amperes: curr,
			IsAc:    isAc,
		})
		if err != nil {
			return res, err
		}
		return data.CaptureResult{
			Type:        data.CaptureTypeTabular,
			TabularData: data.TabularData{Payload: pbReading},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// newPowerCollector returns a collector to register a power method. If one is already registered
// with the same MethodMetadata it will panic.
func newPowerCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, extra map[string]*anypb.Any) (data.CaptureResult, error) {
		var res data.CaptureResult
		pwr, err := ps.Power(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, power.String(), err)
		}
		pbReading, err := uprotoutils.StructToStructPbIgnoreOmitEmpty(pb.GetPowerResponse{
			Watts: pwr,
		})
		if err != nil {
			return res, err
		}
		return data.CaptureResult{
			Type:        data.CaptureTypeTabular,
			TabularData: data.TabularData{Payload: pbReading},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}

// newReadingsCollector returns a collector to register a readings method. If one is already registered
// with the same MethodMetadata it will panic.
func newReadingsCollector(resource interface{}, params data.CollectorParams) (data.Collector, error) {
	ps, err := assertPowerSensor(resource)
	if err != nil {
		return nil, err
	}

	cFunc := data.CaptureFunc(func(ctx context.Context, arg map[string]*anypb.Any) (data.CaptureResult, error) {
		var res data.CaptureResult
		values, err := ps.Readings(ctx, data.FromDMExtraMap)
		if err != nil {
			// A modular filter component can be created to filter the readings from a component. The error ErrNoCaptureToStore
			// is used in the datamanager to exclude readings from being captured and stored.
			if errors.Is(err, data.ErrNoCaptureToStore) {
				return res, err
			}
			return res, data.FailedToReadErr(params.ComponentName, readings.String(), err)
		}
		readings, err := protoutils.ReadingGoToProto(values)
		if err != nil {
			return res, err
		}
		return data.CaptureResult{
			Type: data.CaptureTypeTabular,
			TabularData: data.TabularData{
				Payload: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"readings": structpb.NewStructValue(&structpb.Struct{Fields: readings}),
					},
				},
			},
		}, nil
	})
	return data.NewCollector(cFunc, params)
}
