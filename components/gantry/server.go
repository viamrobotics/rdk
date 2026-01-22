// Package gantry contains a gRPC based gantry service server.
package gantry

import (
	"context"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gantry/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

var unimplemented = "unimplemented"

// serviceServer implements the GantryService from gantry.proto.
type serviceServer struct {
	pb.UnimplementedGantryServiceServer
	coll resource.APIResourceGetter[Gantry]
}

// NewRPCServiceServer constructs an gantry gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Gantry], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// GetPosition returns the position of the gantry specified.
func (s *serviceServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := gantry.Position(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	// if the position is nil, return an empty array.
	if pos == nil {
		pos = []float64{}
	}

	return &pb.GetPositionResponse{PositionsMm: pos}, nil
}

// GetLengths gets the lengths of a gantry of the underlying robot.
func (s *serviceServer) GetLengths(
	ctx context.Context,
	req *pb.GetLengthsRequest,
) (*pb.GetLengthsResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	lengthsMm, err := gantry.Lengths(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	// if the value returned is nil, return an empty array.
	if lengthsMm == nil {
		lengthsMm = []float64{}
	}
	return &pb.GetLengthsResponse{LengthsMm: lengthsMm}, nil
}

// Home runs the homing sequence of the gantry and returns true once completed.
func (s *serviceServer) Home(
	ctx context.Context,
	req *pb.HomeRequest,
) (*pb.HomeResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	homed, err := gantry.Home(ctx, req.Extra.AsMap())
	if err != nil {
		return &pb.HomeResponse{Homed: homed}, err
	}
	return &pb.HomeResponse{Homed: homed}, nil
}

// MoveToPosition moves the gantry to the position specified.
func (s *serviceServer) MoveToPosition(
	ctx context.Context,
	req *pb.MoveToPositionRequest,
) (*pb.MoveToPositionResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.MoveToPositionResponse{}, gantry.MoveToPosition(ctx, req.PositionsMm, req.SpeedsMmPerSec, req.Extra.AsMap())
}

// Stop stops the gantry specified.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, gantry.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gantry, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := gantry.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

func (s *serviceServer) GetKinematics(ctx context.Context, req *commonpb.GetKinematicsRequest) (*commonpb.GetKinematicsResponse, error) {
	gantry, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	model, err := gantry.Kinematics(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.KinematicModelToProtobuf(model), nil
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		// if the error tells us the method is unimplemented, then we
		// can use the kinematics and joint positions endpoints to
		// construct the geometries of the gantry
		if strings.Contains(err.Error(), unimplemented) {
			kinematicsPbResp, err := s.GetKinematics(ctx, &commonpb.GetKinematicsRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			model, err := referenceframe.KinematicModelFromProtobuf(req.GetName(), kinematicsPbResp)
			if err != nil {
				return nil, err
			}

			posResp, err := s.GetPosition(ctx, &pb.GetPositionRequest{Name: req.GetName()})
			if err != nil {
				return nil, err
			}
			gifs, err := model.Geometries(posResp.PositionsMm)
			if err != nil {
				return nil, err
			}
			return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(
				gifs.Geometries())}, nil
		}
		return nil, err
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gantry, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, gantry, req)
}
