package encoder

import (
	"context"

	"github.com/pkg/errors"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/encoder/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/subtype"
)

type subtypeServer struct {
	pb.UnimplementedEncoderServiceServer
	s subtype.Service
}

// NewServer constructs an Encoder gRPC service subtypeServer.
func NewServer(s subtype.Service) pb.EncoderServiceServer {
	return &subtypeServer{s: s}
}

// getEncoder returns the specified encoder or nil.
func (s *subtypeServer) getEncoder(name string) (Encoder, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no Encoder with name (%s)", name)
	}
	enc, ok := resource.(Encoder)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not an Encoder", name)
	}
	return enc, nil
}

// GetPosition returns the current position in terms of ticks or
// degrees, and whether it is a relative or absolute position.
func (s *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	enc, err := s.getEncoder(req.Name)
	if err != nil {
		return nil, err
	}
	posType := ToEncoderPositionType(req.PositionType)
	position, positionType, err := enc.GetPosition(ctx, &posType, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	posType1 := ToProtoPositionType(&positionType)
	return &pb.GetPositionResponse{
		Value:        float32(position),
		PositionType: posType1,
	}, nil
}

// ResetPosition sets the current position of the encoder
// specified by the request to be its new zero position.
func (s *subtypeServer) ResetPosition(
	ctx context.Context,
	req *pb.ResetPositionRequest,
) (*pb.ResetPositionResponse, error) {
	encName := req.GetName()
	enc, err := s.getEncoder(encName)
	if err != nil {
		return nil, errors.Errorf("no encoder (%s) found", encName)
	}

	return &pb.ResetPositionResponse{}, enc.ResetPosition(ctx, req.Extra.AsMap())
}

// GetProperties returns a message of booleans indicating which optional features the robot's encoder supports.
func (s *subtypeServer) GetProperties(
	ctx context.Context,
	req *pb.GetPropertiesRequest,
) (*pb.GetPropertiesResponse, error) {
	encoderName := req.GetName()
	enc, err := s.getEncoder(encoderName)
	if err != nil {
		return nil, errors.Errorf("no encoder (%s) found", encoderName)
	}
	features, err := enc.GetProperties(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return FeatureMapToProtoResponse(features)
}

// DoCommand receives arbitrary commands.
func (s *subtypeServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	enc, err := s.getEncoder(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, enc, req)
}
