// Package sensor contains a gRPC based Sensor service serviceServer.
package sensor

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/sensor/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the SensorService from sensor.proto.
type serviceServer struct {
	pb.UnimplementedSensorServiceServer
	coll resource.APIResourceCollection[Sensor]
}

// NewRPCServiceServer constructs an sensor gRPC service serviceServer.
func NewRPCServiceServer(coll resource.APIResourceCollection[Sensor]) interface{} {
	return &serviceServer{coll: coll}
}

// GetReadings returns the most recent readings from the given Sensor.
func (s *serviceServer) GetReadings(
	ctx context.Context,
	req *commonpb.GetReadingsRequest,
) (*commonpb.GetReadingsResponse, error) {
	sensorDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	readings, err := sensorDevice.Readings(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	m, err := protoutils.ReadingGoToProto(readings)
	if err != nil {
		return nil, err
	}
	return &commonpb.GetReadingsResponse{Readings: m}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	sensorDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, sensorDevice, req)
}
