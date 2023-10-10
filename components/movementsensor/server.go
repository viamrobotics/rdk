package movementsensor

import (
	"context"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/movementsensor/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

type serviceServer struct {
	pb.UnimplementedMovementSensorServiceServer
	coll resource.APIResourceCollection[MovementSensor]
}

// NewRPCServiceServer constructs an MovementSensor gRPC service serviceServer.
func NewRPCServiceServer(coll resource.APIResourceCollection[MovementSensor]) interface{} {
	return &serviceServer{coll: coll}
}

func (s *serviceServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	loc, altitide, err := msDevice.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{
		Coordinate: &commonpb.GeoPoint{Latitude: loc.Lat(), Longitude: loc.Lng()},
		AltitudeM:  float32(altitide),
	}, nil
}

func (s *serviceServer) GetLinearVelocity(
	ctx context.Context,
	req *pb.GetLinearVelocityRequest,
) (*pb.GetLinearVelocityResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
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

func (s *serviceServer) GetAngularVelocity(
	ctx context.Context,
	req *pb.GetAngularVelocityRequest,
) (*pb.GetAngularVelocityResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
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

func (s *serviceServer) GetLinearAcceleration(
	ctx context.Context,
	req *pb.GetLinearAccelerationRequest,
) (*pb.GetLinearAccelerationResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	la, err := msDevice.LinearAcceleration(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetLinearAccelerationResponse{
		LinearAcceleration: protoutils.ConvertVectorR3ToProto(la),
	}, nil
}

func (s *serviceServer) GetCompassHeading(
	ctx context.Context,
	req *pb.GetCompassHeadingRequest,
) (*pb.GetCompassHeadingResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
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

func (s *serviceServer) GetOrientation(
	ctx context.Context,
	req *pb.GetOrientationRequest,
) (*pb.GetOrientationResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
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

func (s *serviceServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	prop, err := msDevice.Properties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return PropertiesToProtoResponse(prop)
}

func (s *serviceServer) GetAccuracy(
	ctx context.Context,
	req *pb.GetAccuracyRequest,
) (*pb.GetAccuracyResponse, error) {
	msDevice, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	acc, err := msDevice.Accuracy(ctx, req.Extra.AsMap())
	return &pb.GetAccuracyResponse{Accuracy: acc}, err
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	msDevice, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, msDevice, req)
}
