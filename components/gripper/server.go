// Package gripper contains a gRPC based gripper service server.
package gripper

import (
	"context"
	"math"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/gripper/v1"
	"go.viam.com/utils/protoutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rdkgrpc "go.viam.com/rdk/grpc"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	rprotoutils "go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
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

// IsHoldingSomething queries if the gripper has managed to grab something.
func (s *serviceServer) IsHoldingSomething(ctx context.Context, req *pb.IsHoldingSomethingRequest) (*pb.IsHoldingSomethingResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	holdingStatus, err := gripper.IsHoldingSomething(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	meta, err := protoutils.StructToStructPb(holdingStatus.Meta)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	inputs, err := gripper.CurrentInputs(ctx)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	inputs := make([]referenceframe.Input, len(req.Values))
	for i, v := range req.Values {
		inputs[i] = referenceframe.Input(v)
	}
	return &pb.GoToInputsResponse{}, gripper.GoToInputs(ctx, inputs)
}

// percentOpenFallbackThreshold is the opening at and above which SetPercentOpen falls back to Open
// (and below which it falls back to Grab) on a gripper that does not support SetPercentOpen.
const percentOpenFallbackThreshold = 0.5

// SetPercentOpen moves the gripper to a normalized opening, from 0.0 (closed) to 1.0 (open).
// If the gripper does not support it (Properties reports PercentOpenSupported = false or is
// unimplemented, or SetPercentOpen is unimplemented), it falls back to Grab for a percentOpen below
// 0.5 and Open otherwise, so every gripper accepts the call.
func (s *serviceServer) SetPercentOpen(
	ctx context.Context,
	req *pb.SetPercentOpenRequest,
) (*pb.SetPercentOpenResponse, error) {
	operation.CancelOtherWithLabel(ctx, req.GetName())
	percentOpen := req.GetPercentOpen()
	if math.IsNaN(percentOpen) || percentOpen < 0 || percentOpen > 1 {
		return nil, status.Errorf(codes.InvalidArgument, "percent_open must be between 0.0 and 1.0, got %v", percentOpen)
	}
	g, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	extra := req.Extra.AsMap()

	props, err := g.Properties(ctx, extra)
	switch {
	case err == nil && props.PercentOpenSupported:
		err = g.SetPercentOpen(ctx, percentOpen, extra)
		if !rdkgrpc.IsUnimplementedError(err) {
			return &pb.SetPercentOpenResponse{}, err
		}
	case err != nil && !rdkgrpc.IsUnimplementedError(err):
		return nil, err
	}

	if percentOpen < percentOpenFallbackThreshold {
		if _, err := g.Grab(ctx, extra); err != nil {
			return nil, err
		}
		return &pb.SetPercentOpenResponse{}, nil
	}
	return &pb.SetPercentOpenResponse{}, g.Open(ctx, extra)
}

// GetPercentOpen returns the measured opening of the gripper, from 0.0 (closed) to 1.0 (open).
func (s *serviceServer) GetPercentOpen(
	ctx context.Context,
	req *pb.GetPercentOpenRequest,
) (*pb.GetPercentOpenResponse, error) {
	g, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	percentOpen, err := g.PercentOpen(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPercentOpenResponse{PercentOpen: percentOpen}, nil
}

// GetProperties returns the capabilities of the gripper.
func (s *serviceServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	g, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	props, err := g.Properties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPropertiesResponse{PercentOpenSupported: props.PercentOpenSupported}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	gripper, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return rprotoutils.DoFromResourceServer(ctx, gripper, req)
}

func (s *serviceServer) GetGeometries(ctx context.Context, req *commonpb.GetGeometriesRequest) (*commonpb.GetGeometriesResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	geometries, err := framesystem.GeometriesWithFallback(ctx, res, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &commonpb.GetGeometriesResponse{Geometries: referenceframe.NewGeometriesToProto(geometries)}, nil
}

func (s *serviceServer) GetKinematics(ctx context.Context, req *commonpb.GetKinematicsRequest) (*commonpb.GetKinematicsResponse, error) {
	g, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	model, err := g.Kinematics(ctx)
	if err != nil {
		return nil, err
	}
	return referenceframe.KinematicsResponse(model, req), nil
}

// GetStatus returns the status of the gripper.
func (s *serviceServer) GetStatus(ctx context.Context, req *commonpb.GetStatusRequest) (*commonpb.GetStatusResponse, error) {
	res, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return rprotoutils.GetStatusFromResourceServer(ctx, res, req)
}
