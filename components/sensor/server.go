// Package sensor contains a gRPC based Sensor service subtypeServer.
package sensor

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/sensor/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the SensorService from sensor.proto.
type subtypeServer struct {
	pb.UnimplementedSensorServiceServer
	s subtype.Service
}

// NewServer constructs an sensor gRPC service subtypeServer.
func NewServer(s subtype.Service) pb.SensorServiceServer {
	return &subtypeServer{s: s}
}

// getSensor returns the sensor specified, nil if not.
func (s *subtypeServer) getSensor(name string) (Sensor, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no generic sensor with name (%s)", name)
	}
	sensor, ok := resource.(Sensor)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a generic sensor", name)
	}
	return sensor, nil
}

// GetReadings returns the most recent readings from the given Sensor.
func (s *subtypeServer) GetReadings(
	ctx context.Context,
	req *pb.GetReadingsRequest,
) (*pb.GetReadingsResponse, error) {
	sensorDevice, err := s.getSensor(req.Name)
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
	sensorDevice, err := s.getSensor(req.Name)
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, sensorDevice, req)
}
