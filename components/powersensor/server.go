package powersensor

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/powersensor/v1"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedPowerSensorServiceServer
	coll resource.APIResourceGetter[PowerSensor]
}

// NewRPCServiceServer constructs a PowerSesnsor gRPC service serviceServer.
func NewRPCServiceServer(coll resource.APIResourceGetter[PowerSensor], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// GetReadings returns the most recent readings from the given Sensor.
func (s *serviceServer) GetReadings(
	ctx context.Context,
	req *commonpb.GetReadingsRequest,
) (*commonpb.GetReadingsResponse, error) {
	sensorDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	readings, err := sensorDevice.Readings(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	if readings == nil {
		return nil, errtrace.Wrap(sensor.ErrReadingsNil("power-sensor", req.Name))
	}
	m, err := protoutils.ReadingGoToProto(readings)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &commonpb.GetReadingsResponse{Readings: m}, nil
}

func (s *serviceServer) GetVoltage(
	ctx context.Context,
	req *pb.GetVoltageRequest,
) (*pb.GetVoltageResponse, error) {
	psDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	voltage, isAc, err := psDevice.Voltage(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.GetVoltageResponse{
		Volts: voltage,
		IsAc:  isAc,
	}, nil
}

func (s *serviceServer) GetCurrent(
	ctx context.Context,
	req *pb.GetCurrentRequest,
) (*pb.GetCurrentResponse, error) {
	psDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	current, isAc, err := psDevice.Current(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.GetCurrentResponse{
		Amperes: current,
		IsAc:    isAc,
	}, nil
}

func (s *serviceServer) GetPower(
	ctx context.Context,
	req *pb.GetPowerRequest,
) (*pb.GetPowerResponse, error) {
	psDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	power, err := psDevice.Power(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.GetPowerResponse{
		Watts: power,
	}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	psDevice, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.DoFromResourceServer(ctx, psDevice, req))
}

// GetStatus returns the status of the power sensor.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(protoutils.GetStatusFromResourceServer(ctx, res, req))
}
