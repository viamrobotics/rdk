// Package servo contains a gRPC based servo service server
package servo

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/component/servo/v1"
	"go.viam.com/rdk/subtype"
)

type subtypeServer struct {
	pb.UnimplementedServoServiceServer
	service subtype.Service
}

// NewServer constructs a servo gRPC service server.
func NewServer(service subtype.Service) pb.ServoServiceServer {
	return &subtypeServer{service: service}
}

// getServo returns the specified servo or nil.
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

func (server *subtypeServer) Move(ctx context.Context, req *pb.MoveRequest) (*pb.MoveResponse, error) {
	operation.CancelOtherWithLabel(ctx, "base-actuate-"+req.GetName())
	servo, err := server.getServo(req.GetName())
	if err != nil {
		return nil, err
	}
	return &pb.MoveResponse{}, servo.Move(ctx, uint8(req.GetAngleDeg()))
}

func (server *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	servo, err := server.getServo(req.GetName())
	if err != nil {
		return nil, err
	}
	angleDeg, err := servo.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{PositionDeg: uint32(angleDeg)}, nil
}
