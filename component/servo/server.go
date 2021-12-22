// Package servo contains a gRPC based servo service server
package servo

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/core/proto/api/component/v1"
	"go.viam.com/core/subtype"
)

type subtypeServer struct {
	pb.UnimplementedServoServiceServer
	service subtype.Service
}

// NewServer constructs a servo gRPC service server
func NewServer(service subtype.Service) pb.ServoServiceServer {
	return &subtypeServer{service: service}
}

// getServo returns the specified servo or nil
func (server *subtypeServer) getServo(name string) (Servo, error) {
	resource := server.service.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no servo with name (%s)", name)
	}
	servo, ok := resource.(Servo)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a servo", name)
	}
	return servo, nil
}

func (server *subtypeServer) Move(ctx context.Context, req *pb.ServoServiceMoveRequest) (*pb.ServoServiceMoveResponse, error) {
	servo, err := server.getServo(req.GetName())
	if err != nil {
		return nil, err
	}
	return &pb.ServoServiceMoveResponse{}, servo.Move(ctx, uint8(req.GetAngleDeg()))
}

func (server *subtypeServer) AngularOffset(
	ctx context.Context,
	req *pb.ServoServiceAngularOffsetRequest,
) (*pb.ServoServiceAngularOffsetResponse, error) {
	servo, err := server.getServo(req.GetName())
	if err != nil {
		return nil, err
	}
	angleDeg, err := servo.AngularOffset(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.ServoServiceAngularOffsetResponse{AngleDeg: uint32(angleDeg)}, nil
}
