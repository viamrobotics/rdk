// Package sensor contains a gRPC based Sensor service subtypeServer.
package sensor

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from sensor_subtype.proto.
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
		return nil, errors.Errorf("no Sensor with name (%s)", name)
	}
	sensor, ok := resource.(Sensor)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a Sensor", name)
	}
	return sensor, nil
}

// Readings returns the most recent readings from the given Sensor.
func (s *subtypeServer) Readings(ctx context.Context, req *pb.SensorServiceReadingsRequest) (*pb.SensorServiceReadingsResponse, error) {
	sensorDevice, err := s.getSensor(req.Name)
	if err != nil {
		return nil, err
	}
	readings, err := sensorDevice.Readings(ctx)
	if err != nil {
		return nil, err
	}
	readingsP := make([]*structpb.Value, 0, len(readings))
	for _, r := range readings {
		v, err := structpb.NewValue(r)
		if err != nil {
			return nil, err
		}
		readingsP = append(readingsP, v)
	}
	return &pb.SensorServiceReadingsResponse{Readings: readingsP}, nil
}

// Desc returns the most recent Desc from the given Sensor.
func (s *subtypeServer) Desc(ctx context.Context, req *pb.SensorServiceDescRequest) (*pb.SensorServiceDescResponse, error) {
	sensorDevice, err := s.getSensor(req.Name)
	if err != nil {
		return nil, err
	}
	desc, err := sensorDevice.Desc(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.SensorServiceDescResponse{
		Desc: &commonpb.SensorDescription{Type: string(desc.Type), Path: desc.Path},
	}, nil
}
