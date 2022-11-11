package movementsensor

import (
	"context"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"

	"go.viam.com/rdk/protoutils"
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
	loc, altitide, err := msDevice.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{
		Coordinate: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
		AltitudeMm: float32(altitide),
	}, nil
}

func (s *subtypeServer) GetLinearVelocity(
	ctx context.Context,
	req *pb.GetLinearVelocityRequest,
) (*pb.GetLinearVelocityResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	vel, err := msDevice.LinearVelocity(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetLinearVelocityResponse{
		LinearVelocity: protoutils.ConvertVectorR3ToProto(vel),
	}, nil
}

func (s *subtypeServer) GetAngularVelocity(
	ctx context.Context,
	req *pb.GetAngularVelocityRequest,
) (*pb.GetAngularVelocityResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	av, err := msDevice.AngularVelocity(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetAngularVelocityResponse{
		AngularVelocity: protoutils.ConvertVectorR3ToProto(r3.Vector(av)),
	}, nil
}

func (s *subtypeServer) GetCompassHeading(
	ctx context.Context,
	req *pb.GetCompassHeadingRequest,
) (*pb.GetCompassHeadingResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	ch, err := msDevice.CompassHeading(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetCompassHeadingResponse{
		Value: ch,
	}, nil
}

func (s *subtypeServer) GetOrientation(
	ctx context.Context,
	req *pb.GetOrientationRequest,
) (*pb.GetOrientationResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	ori, err := msDevice.Orientation(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetOrientationResponse{
		Orientation: protoutils.ConvertOrientationToProto(ori),
	}, nil
}

func (s *subtypeServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	prop, err := msDevice.Properties(ctx, req.Extra.AsMap())
	return (*pb.GetPropertiesResponse)(prop), err
}

func (s *subtypeServer) GetAccuracy(
	ctx context.Context,
	req *pb.GetAccuracyRequest,
) (*pb.GetAccuracyResponse, error) {
	msDevice, err := s.getMovementSensor(req.Name)
	if err != nil {
		return nil, err
	}
	acc, err := msDevice.Accuracy(ctx, req.Extra.AsMap())
	return &pb.GetAccuracyResponse{AccuracyMm: acc}, err
}
