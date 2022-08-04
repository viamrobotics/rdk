package movementsensor

import (
	"context"

	"github.com/pkg/errors"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	pb "go.viam.com/rdk/proto/api/component/movementsensor/v1"
	"go.viam.com/rdk/subtype"
)

type subtypeServer struct {
	pb.UnimplementedMovementSensorServiceServer
	s subtype.Service
}

// NewServer constructs an MovementSensor gRPC service subtypeServer.
func NewServer(s subtype.Service) pb.MovementSensorServiceServer {
	return &subtypeServer{s: s}
}

func (s *subtypeServer) getMovementSensor(name string) (MovementSensor, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no MovementSensor with name (%s)", name)
	}
	ms, ok := resource.(MovementSensor)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a MovementSensor", name)
	}
	return ms, nil
}

func (s *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	loc, altitide, accuracy, err := msDevice.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{
		Coordinate: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
		AltitudeMm: float32(altitide),
		AccuracyMm: float32(accuracy),
	}, nil
}


