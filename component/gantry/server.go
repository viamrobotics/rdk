// Package gantry contains a gRPC based gantry service server.
package gantry

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from gantry.proto.
type subtypeServer struct {
	pb.UnimplementedGantryServiceServer
	s subtype.Service
}

// NewServer constructs an gantry gRPC service server.
func NewServer(s subtype.Service) pb.GantryServiceServer {
	return &subtypeServer{s: s}
}

// getGantry returns the gantry specified, nil if not.
func (s *subtypeServer) getGantry(name string) (Gantry, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no gantry with name (%s)", name)
	}
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a gantry", name)
	}
	return gantry, nil
}

// CurrentPosition returns the position of the gantry specified.
func (s *subtypeServer) CurrentPosition(
	ctx context.Context,
	req *pb.GantryServiceCurrentPositionRequest,
) (*pb.GantryServiceCurrentPositionResponse, error) {
	gantry, err := s.getGantry(req.Name)
	if err != nil {
		return nil, err
	}
	pos, err := gantry.CurrentPosition(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GantryServiceCurrentPositionResponse{Positions: pos}, nil
}

// GetLengths gets the lengths of a gantry of the underlying robot.
func (s *subtypeServer) GetLengths(
	ctx context.Context,
	req *pb.GantryServiceGetLengthsRequest,
) (*pb.GantryServiceGetLengthsResponse, error) {
	gantry, err := s.getGantry(req.Name)
	if err != nil {
		return nil, err
	}
	lengthsMm, err := gantry.GetLengths(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GantryServiceGetLengthsResponse{LengthsMm: lengthsMm}, nil
}

// MoveToPosition moves the gantry to the position specified.
func (s *subtypeServer) MoveToPosition(
	ctx context.Context,
	req *pb.GantryServiceMoveToPositionRequest,
) (*pb.GantryServiceMoveToPositionResponse, error) {
	gantry, err := s.getGantry(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.GantryServiceMoveToPositionResponse{}, gantry.MoveToPosition(ctx, req.PositionsMm)
}
