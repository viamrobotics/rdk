// Package sensor contains a gRPC based Sensor service subtypeServer.
package sensor

import (
	"context"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"

	pb "go.viam.com/rdk/proto/api/component/sensor/v1"
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
	readings, err := sensorDevice.GetReadings(ctx)
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
	return &pb.GetReadingsResponse{Readings: readingsP}, nil
}
