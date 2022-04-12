// Package motor contains a gRPC based motor service server
package motor

import (
	"context"

	"github.com/pkg/errors"

	"go.viam.com/rdk/operation"
	pb "go.viam.com/rdk/proto/api/component/motor/v1"
	"go.viam.com/rdk/subtype"
)

type subtypeServer struct {
	pb.UnimplementedMotorServiceServer
	service subtype.Service
}

// NewServer constructs a motor gRPC service server.
func NewServer(service subtype.Service) pb.MotorServiceServer {
	return &subtypeServer{service: service}
}

// getMotor returns the specified motor or nil.
func (server *subtypeServer) getMotor(name string) (Motor, error) {
	resource := server.service.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no motor with name (%s)", name)
	}
	motor, ok := resource.(Motor)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a motor", name)
	}
	return motor, nil
}

// SetPower sets the percentage of power the motor of the underlying robot should employ between 0-1.
func (server *subtypeServer) SetPower(
	ctx context.Context,
	req *pb.SetPowerRequest,
) (*pb.SetPowerResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}
	return &pb.SetPowerResponse{}, motor.SetPower(ctx, req.GetPowerPct())
}

// GoFor requests the motor of the underlying robot to go for a certain amount based off
// the request.
func (server *subtypeServer) GoFor(
	ctx context.Context,
	req *pb.GoForRequest,
) (*pb.GoForResponse, error) {
	operation.CancelOtherWithLabel(ctx, "motor-actuate-"+req.GetName())
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	// erh: this isn't right semantically.
	// GoFor with 0 rotations means something important.
	rVal := 0.0
	revolutions := req.GetRevolutions()
	if revolutions != 0 {
		rVal = revolutions
	}

	return &pb.GoForResponse{}, motor.GoFor(ctx, req.GetRpm(), rVal)
}

// GetPosition reports the position of the motor of the underlying robot
// based on its encoder. If it's not supported, the returned data is undefined.
// The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (server *subtypeServer) GetPosition(
	ctx context.Context,
	req *pb.GetPositionRequest,
) (*pb.GetPositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	pos, err := motor.GetPosition(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.GetPositionResponse{Position: pos}, nil
}

// GetFeatures returns a message of booleans indicating which optional features the robot's motor supports.
func (server *subtypeServer) GetFeatures(
	ctx context.Context,
	req *pb.GetFeaturesRequest,
) (*pb.GetFeaturesResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}
	features, err := motor.GetFeatures(ctx)
	if err != nil {
		return nil, err
	}
	return FeatureMapToProtoResponse(features)
}

// Stop turns the motor of the underlying robot off.
func (server *subtypeServer) Stop(
	ctx context.Context,
	req *pb.StopRequest,
) (*pb.StopResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.StopResponse{}, motor.Stop(ctx)
}

// IsPowered returns whether or not the motor of the underlying robot is currently on.
func (server *subtypeServer) IsPowered(
	ctx context.Context,
	req *pb.IsPoweredRequest,
) (*pb.IsPoweredResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	isOn, err := motor.IsPowered(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.IsPoweredResponse{IsOn: isOn}, nil
}

// GoTo requests the motor of the underlying robot to go a specific position.
func (server *subtypeServer) GoTo(
	ctx context.Context,
	req *pb.GoToRequest,
) (*pb.GoToResponse, error) {
	operation.CancelOtherWithLabel(ctx, "motor-actuate-"+req.GetName())
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.GoToResponse{}, motor.GoTo(ctx, req.GetRpm(), req.GetPositionRevolutions())
}

// ResetZeroPosition sets the current position of the motor specified by the request
// (adjusted by a given offset) to be its new zero position.
func (server *subtypeServer) ResetZeroPosition(
	ctx context.Context,
	req *pb.ResetZeroPositionRequest,
) (*pb.ResetZeroPositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.ResetZeroPositionResponse{}, motor.ResetZeroPosition(ctx, req.GetOffset())
}
