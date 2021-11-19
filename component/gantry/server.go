// Package gantry contains a gRPC based gantry service server.
package gantry

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/subtype"
)

// SubtypeServer implements the contract from gantry.proto
type SubtypeServer struct {
	pb.UnimplementedGantryServiceServer
	s subtype.Service
}

// NewServer constructs an gantry gRPC service server.
func NewServer(s subtype.Service) pb.GantryServiceServer {
	return &SubtypeServer{s: s}
}

// getGantry returns the gantry specified, nil if not.
func (s *SubtypeServer) getGantry(name string) (Gantry, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no gantry with name (%s)", name)
	}
	gantry, ok := resource.(Gantry)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not an gantry", name)
	}
	return gantry, nil
}

// CurrentPosition returns the position of the gantry specified.
func (s *SubtypeServer) CurrentPosition(ctx context.Context, req *pb.GantryServiceCurrentPositionRequest) (*pb.GantryServiceCurrentPositionResponse, error) {
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

// Lengths gets the lengths of a gantry of the underlying robot.
func (s *SubtypeServer) Lengths(ctx context.Context, req *pb.GantryServiceLengthsRequest) (*pb.GantryServiceLengthsResponse, error) {
	gantry, err := s.getGantry(req.Name)
	if err != nil {
		return nil, err
	}
	lengths, err := gantry.Lengths(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GantryServiceLengthsResponse{Lengths: lengths}, nil
}

// MoveToPosition returns the position of the gantry specified.
func (s *SubtypeServer) MoveToPosition(ctx context.Context, req *pb.GantryServiceMoveToPositionRequest) (*pb.GantryServiceMoveToPositionResponse, error) {
	gantry, err := s.getGantry(req.Name)
	if err != nil {
		return nil, err
	}
	return &pb.GantryServiceMoveToPositionResponse{}, gantry.MoveToPosition(ctx, req.Positions)
}
