// Package motor contains a gRPC based motor service server
package motor

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"

	pb "go.viam.com/rdk/proto/api/component/v1"
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

// GetPIDConfig returns the config of the motor's PID.
func (server *subtypeServer) GetPIDConfig(
	ctx context.Context,
	req *pb.MotorServiceGetPIDConfigRequest,
) (*pb.MotorServiceGetPIDConfigResponse, error) {
	return nil, errors.New("motorGetPidNotImpl")
}

func (server *subtypeServer) SetPIDConfig(
	ctx context.Context,
	req *pb.MotorServiceSetPIDConfigRequest,
) (*pb.MotorServiceSetPIDConfigResponse, error) {
	return nil, errors.New("motorGetPidNotImpl")
}

// PIDStep execute a step response on the PID controller.
func (server *subtypeServer) PIDStep(
	req *pb.MotorServicePIDStepRequest,
	serverPIDStep pb.MotorService_PIDStepServer,
) (result error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return errors.Errorf("no motor (%s) found", motorName)
	}
	setPoint := req.GetSetPoint()
	if err := motor.Stop(serverPIDStep.Context()); err != nil {
		return err
	}

	lastTime := time.Now()
	lastPos, err := motor.Position(serverPIDStep.Context())
	totalTime := 0.0
	if err != nil {
		return err
	}
	ticker := time.NewTicker(time.Millisecond * 10)
	defer ticker.Stop()
	defer func(m Motor, err error) {
		// TODO - previous version had logging, but used the logger from the robot,
		// should we still try to do this? - GV
		errOff := m.Stop(serverPIDStep.Context())
		if errOff != nil {
			result = multierr.Combine(errOff, err)
		}
	}(motor, err)

	for {
		select {
		case <-serverPIDStep.Context().Done():
			err := motor.Stop(serverPIDStep.Context())
			return multierr.Combine(serverPIDStep.Context().Err(), err)
		default:
		}
		<-ticker.C
		dt := time.Since(lastTime)
		lastTime = time.Now()
		currPos, err := motor.Position(serverPIDStep.Context())
		if err != nil {
			return err
		}
		vel := (currPos - lastPos) / dt.Seconds()
		lastPos = currPos

		totalTime += dt.Seconds()
		if err := serverPIDStep.Send(&pb.MotorServicePIDStepResponse{Time: totalTime, SetPoint: setPoint, RefValue: vel}); err != nil {
			return err
		}
	}
}

// SetPower sets the percentage of power the motor of the underlying robot should employ between 0-1.
func (server *subtypeServer) SetPower(
	ctx context.Context,
	req *pb.MotorServiceSetPowerRequest,
) (*pb.MotorServiceSetPowerResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}
	return &pb.MotorServiceSetPowerResponse{}, motor.SetPower(ctx, req.GetPowerPct())
}

// Go requests the motor of the underlying robot to go.
func (server *subtypeServer) Go(
	ctx context.Context,
	req *pb.MotorServiceGoRequest,
) (*pb.MotorServiceGoResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}
	return &pb.MotorServiceGoResponse{}, motor.Go(ctx, req.GetPowerPct())
}

// GoFor requests the motor of the underlying robot to go for a certain amount based off
// the request.
func (server *subtypeServer) GoFor(
	ctx context.Context,
	req *pb.MotorServiceGoForRequest,
) (*pb.MotorServiceGoForResponse, error) {
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

	return &pb.MotorServiceGoForResponse{}, motor.GoFor(ctx, req.GetRpm(), rVal)
}

// Position reports the position of the motor of the underlying robot
// based on its encoder. If it's not supported, the returned data is undefined.
// The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (server *subtypeServer) Position(
	ctx context.Context,
	req *pb.MotorServicePositionRequest,
) (*pb.MotorServicePositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	pos, err := motor.Position(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorServicePositionResponse{Position: pos}, nil
}

// PositionSupported returns whether or not the motor of the underlying robot supports reporting of its position which
// is reliant on having an encoder.
func (server *subtypeServer) PositionSupported(
	ctx context.Context,
	req *pb.MotorServicePositionSupportedRequest,
) (*pb.MotorServicePositionSupportedResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	supported, err := motor.PositionSupported(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorServicePositionSupportedResponse{Supported: supported}, nil
}

// Stop turns the motor of the underlying robot off.
func (server *subtypeServer) Stop(
	ctx context.Context,
	req *pb.MotorServiceStopRequest,
) (*pb.MotorServiceStopResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.MotorServiceStopResponse{}, motor.Stop(ctx)
}

// IsOn returns whether or not the motor of the underlying robot is currently on.
func (server *subtypeServer) IsOn(
	ctx context.Context,
	req *pb.MotorServiceIsOnRequest,
) (*pb.MotorServiceIsOnResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	isOn, err := motor.IsOn(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.MotorServiceIsOnResponse{IsOn: isOn}, nil
}

// GoTo requests the motor of the underlying robot to go a specific position.
func (server *subtypeServer) GoTo(
	ctx context.Context,
	req *pb.MotorServiceGoToRequest,
) (*pb.MotorServiceGoToResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.MotorServiceGoToResponse{}, motor.GoTo(ctx, req.GetRpm(), req.GetPosition())
}

// GoTillStop requests the motor of the underlying robot to go until stopped either physically or by a limit switch.
func (server *subtypeServer) GoTillStop(
	ctx context.Context,
	req *pb.MotorServiceGoTillStopRequest,
) (*pb.MotorServiceGoTillStopResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.MotorServiceGoTillStopResponse{}, motor.GoTillStop(ctx, req.GetRpm(), nil)
}

// ResetZeroPosition sets the current position of the motor specified by the request
// (adjusted by a given offset) to be its new zero position.
func (server *subtypeServer) ResetZeroPosition(
	ctx context.Context,
	req *pb.MotorServiceResetZeroPositionRequest,
) (*pb.MotorServiceResetZeroPositionResponse, error) {
	motorName := req.GetName()
	motor, err := server.getMotor(motorName)
	if err != nil {
		return nil, errors.Errorf("no motor (%s) found", motorName)
	}

	return &pb.MotorServiceResetZeroPositionResponse{}, motor.ResetZeroPosition(ctx, req.GetOffset())
}
