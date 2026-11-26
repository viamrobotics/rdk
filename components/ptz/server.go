// Package ptz contains a gRPC based PTZ service server.
package ptz

import (
	"context"
	"errors"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/ptz/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// serviceServer implements the PTZService from ptz.proto.
type serviceServer struct {
	pb.UnimplementedPTZServiceServer
	coll resource.APIResourceGetter[PTZ]
}

// NewRPCServiceServer constructs a PTZ gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[PTZ], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// GetStatus returns the current PTZ position and movement status.
func (s *serviceServer) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	ptz, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	status, err := ptz.GetStatus(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, errors.New("ptz returned nil status")
	}
	resp := &pb.GetStatusResponse{
		Position:       status.Position,
		PanTiltStatus:  status.PanTiltStatus,
		ZoomStatus:     status.ZoomStatus,
		UtcTime:        status.UtcTime,
	}
	return resp, nil
}

// GetCapabilities returns standardized PTZ capabilities.
func (s *serviceServer) GetCapabilities(
	ctx context.Context, req *pb.GetCapabilitiesRequest,
) (*pb.GetCapabilitiesResponse, error) {
	ptz, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	caps, err := ptz.GetCapabilities(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if caps == nil {
		return nil, errors.New("ptz returned nil capabilities")
	}
	resp := &pb.GetCapabilitiesResponse{
		MoveCapabilities: caps.MoveCapabilities,
		SupportsStatus:   caps.SupportsStatus,
		SupportsStop:     caps.SupportsStop,
	}
	return resp, nil
}

// Stop halts any ongoing PTZ movement.
func (s *serviceServer) Stop(ctx context.Context, req *pb.StopRequest) (*pb.StopResponse, error) {
	ptz, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	if err := ptz.Stop(ctx, req.PanTilt, req.Zoom, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.StopResponse{}, nil
}

// Move executes a PTZ movement command (continuous, relative, or absolute).
func (s *serviceServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	ptz, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}

	cmd := &MoveCommand{
		Continuous: req.GetContinuous(),
		Relative:   req.GetRelative(),
		Absolute:   req.GetAbsolute(),
	}
	if cmd.Continuous == nil && cmd.Relative == nil && cmd.Absolute == nil {
		return nil, errors.New("move request must include a command")
	}

	if err := ptz.Move(ctx, cmd, req.Extra.AsMap()); err != nil {
		return nil, err
	}
	return &pb.MoveResponse{}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(
	ctx context.Context, req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	ptz, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, ptz, req)
}
