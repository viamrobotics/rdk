// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/utils/protoutils"

	"braces.dev/errtrace"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the GripperService from gripper.proto.
type serviceServer struct {
	pb.UnimplementedGripperServiceServer
	coll resource.APIResourceGetter[Gripper]
}

// NewRPCServiceServer constructs an gripper gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Gripper], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// Open opens a gripper of the underlying robot.
func (s *serviceServer) Open(ctx context.Context, req *pb.OpenRequest) (*pb.OpenResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.OpenResponse{}, errtrace.Wrap(gripper.Open(ctx, req.Extra.AsMap()))
}

// Grab requests a gripper of the underlying robot to grab.
func (s *serviceServer) Grab(ctx context.Context, req *pb.GrabRequest) (*pb.GrabResponse, error) {
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	success, err := gripper.Grab(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.GrabResponse{Success: success}, nil
}

// Stop stops the gripper specified.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.Name)
	gripper, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.StopResponse{}, errtrace.Wrap(gripper.Stop(ctx, req.Extra.AsMap()))
}

// IsMoving queries of a component is in motion.
func (s *serviceServer) IsMoving(ctx context.Context, req *pb.IsMovingRequest) (*pb.IsMovingResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	moving, err := gripper.IsMoving(ctx)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.IsMovingResponse{IsMoving: moving}, nil
}

// IsHoldingSomething queries if the gripper has managed to grab something.
func (s *serviceServer) IsHoldingSomething(ctx context.Context, req *pb.IsHoldingSomethingRequest) (*pb.IsHoldingSomethingResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	holdingStatus, err := gripper.IsHoldingSomething(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	meta, err := protoutils.StructToStructPb(holdingStatus.Meta)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &pb.IsHoldingSomethingResponse{IsHoldingSomething: holdingStatus.IsHoldingSomething, Meta: meta}, nil
}

// GetCurrentInputs returns the current input values of the gripper.
func (s *serviceServer) GetCurrentInputs(
	ctx context.Context,
	req *pb.GetCurrentInputsRequest,
) (*pb.GetCurrentInputsResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	inputs, err := gripper.CurrentInputs(ctx)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	values := make([]float64, len(inputs))
	for i, in := range inputs {
		values[i] = float64(in)
	}
	return &pb.GetCurrentInputsResponse{Values: values}, nil
}

// GoToInputs moves the gripper to the given input values.
func (s *serviceServer) GoToInputs(ctx context.Context, req *pb.GoToInputsRequest) (*pb.GoToInputsResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	inputs := make([]referenceframe.Input, len(req.Values))
	for i, v := range req.Values {
		inputs[i] = referenceframe.Input(v)
	}
	return &pb.GoToInputsResponse{}, errtrace.Wrap(gripper.GoToInputs(ctx, inputs))
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(rprotoutils.DoFromResourceServer(ctx, gripper, req))
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	geometries, err := res.Geometries(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

func (s *serviceServer) GetKinematics(ctx context.Context, req *commonpb.GetKinematicsRequest) (*commonpb.GetKinematicsResponse, error) {
	g, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	model, err := g.Kinematics(ctx)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return referenceframe.KinematicModelToProtobuf(model), nil
}

// GetStatus returns the status of the gripper.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	return errtrace.Wrap2(rprotoutils.GetStatusFromResourceServer(ctx, res, req))
}
