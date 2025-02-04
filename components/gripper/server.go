// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"

	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

// ErrGeometriesNil is the returned error if gripper geometries are nil.
var ErrGeometriesNil = func(gripperName string) error {
	return fmt.Errorf("gripper component %v Geometries should not return nil geometries", gripperName)
}

// serviceServer implements the GripperService from gripper.proto.
type serviceServer struct {
	pb.UnimplementedGripperServiceServer
	coll resource.APIResourceCollection[Gripper]
}

// NewRPCServiceServer constructs an gripper gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Gripper]) interface{} {
	return &serviceServer{coll: coll}
}

// Open opens a gripper of the underlying robot.
func (s *serviceServer) Open(ctx context.Context, req *pb.OpenRequest) (*pb.OpenResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.OpenResponse{}, gripper.Open(ctx, req.Extra.AsMap())
}

// Grab requests a gripper of the underlying robot to grab.
func (s *serviceServer) Grab(ctx context.Context, req *pb.GrabRequest) (*pb.GrabResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	success, err := gripper.Grab(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GrabResponse{Success: success}, nil
}

// Stop stops the gripper specified.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, gripper.Stop(ctx, req.Extra.AsMap())
}

// IsMoving queries of a component is in motion.
func (s *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	moving, err := gripper.IsMoving(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, gripper, req)
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if geometries == nil {
		return nil, ErrGeometriesNil(req.GetName())
	}
	return &commonpb.GetGeometriesResponse{Geometries: spatialmath.NewGeometriesToProto(geometries)}, nil
}
