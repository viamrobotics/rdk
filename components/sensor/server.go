// Package sensor contains a gRPC based Sensor service subtypeServer.
package sensor

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/sensor/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// subtypeServer implements the SensorService from sensor.proto.
type subtypeServer struct {
	pb.UnimplementedSensorServiceServer
	coll resource.SubtypeCollection[Sensor]
}

// NewRPCServiceServer constructs an sensor gRPC service subtypeServer.
func NewRPCServiceServer(coll resource.SubtypeCollection[Sensor]) interface{} {
	return &subtypeServer{coll: coll}
}

// GetReadings returns the most recent readings from the given Sensor.
func (s *subtypeServer) GetReadings(
	ctx context.Context,
	req *pb.GetReadingsRequest,
) (*pb.GetReadingsResponse, error) {
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
	return &pb.GetReadingsResponse{Readings: m}, nil
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	sensorDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, sensorDevice, req)
}
