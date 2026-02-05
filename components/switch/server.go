// Package toggleswitch contains a gRPC based switch service server.
package toggleswitch

import (
	"context"
	"errors"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/switch/v1"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

// ErrInvalidPosition is the returned error if switch position is invalid.
var ErrInvalidPosition = func(switchName string, position, maxPosition int) error {
	return fmt.Errorf("switch component %v position %d is invalid (max: %d)", switchName, position, maxPosition)
}

// serviceServer implements the SwitchService from switch.proto.
type serviceServer struct {
	pb.UnimplementedSwitchServiceServer
	coll resource.APIResourceGetter[Switch]
}

// NewRPCServiceServer constructs a switch gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceGetter[Switch], logger logging.Logger) interface{} {
	return &serviceServer{coll: coll}
}

// SetPosition sets the position of a switch of the underlying robot.
func (s *serviceServer) SetPosition(ctx context.Context, req *pb.SetPositionRequest) (*pb.SetPositionResponse, error) {
	sw, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.SetPositionResponse{}, sw.SetPosition(ctx, req.Position, req.Extra.AsMap())
}

// GetPosition gets the current position of a switch of the underlying robot.
func (s *serviceServer) GetPosition(ctx context.Context, req *pb.GetPositionRequest) (*pb.GetPositionResponse, error) {
	sw, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	position, err := sw.GetPosition(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{Position: position}, nil
}

// GetNumberOfPositions gets the total number of positions for a switch of the underlying robot.
func (s *serviceServer) GetNumberOfPositions(
	ctx context.Context, req *pb.GetNumberOfPositionsRequest,
) (*pb.GetNumberOfPositionsResponse, error) {
	sw, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}
	count, labels, err := sw.GetNumberOfPositions(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	if len(labels) > 0 && len(labels) != int(count) {
		return nil, errors.New("the number of labels does not match the number of positions")
	}
	return &pb.GetNumberOfPositionsResponse{NumberOfPositions: count, Labels: labels}, nil
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	sw, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, sw, req)
}
