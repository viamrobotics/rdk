package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.viam.com/robotcore/api"
	pb "go.viam.com/robotcore/proto/api/v1"
	"go.viam.com/robotcore/robot/actions"
)

type Server struct {
	pb.UnimplementedRobotServiceServer
	r api.Robot
}

func New(r api.Robot) pb.RobotServiceServer {
	return &Server{r: r}
}

func (s *Server) Status(ctx context.Context, _ *pb.StatusRequest) (*pb.StatusResponse, error) {
	status, err := s.r.Status()
	if err != nil {
		return nil, err
	}
	return &pb.StatusResponse{Status: status}, nil
}

const defaultStreamInterval = 1 * time.Second

func (s *Server) StatusStream(req *pb.StatusStreamRequest, server pb.RobotService_StatusStreamServer) error {
	every := defaultStreamInterval
	if reqEvery := req.Every.AsDuration(); reqEvery != time.Duration(0) {
		every = reqEvery
	}
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		default:
		}
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case <-ticker.C:
		}
		status, err := s.r.Status()
		if err != nil {
			return err
		}
		if err := server.Send(&pb.StatusStreamResponse{Status: status}); err != nil {
			return err
		}
	}
}

func (s *Server) DoAction(ctx context.Context, req *pb.DoActionRequest) (*pb.DoActionResponse, error) {
	action := actions.LookupAction(req.Name)
	if action == nil {
		return nil, fmt.Errorf("unknown action name [%s]", req.Name)
	}
	go action(s.r)
	return &pb.DoActionResponse{}, nil
}

func (s *Server) ControlBase(ctx context.Context, req *pb.ControlBaseRequest) (*pb.ControlBaseResponse, error) {
	base := s.r.BaseByName(req.Name)
	if base == nil {
		return nil, fmt.Errorf("no base with name (%s)", req.Name)
	}

	switch v := req.Action.(type) {
	case *pb.ControlBaseRequest_Stop:
		if v.Stop {
			return &pb.ControlBaseResponse{}, base.Stop(ctx)
		}
		return &pb.ControlBaseResponse{}, nil
	case *pb.ControlBaseRequest_Move:
		if v.Move == nil {
			return &pb.ControlBaseResponse{}, errors.New("move unspecified")
		}
		millisPerSec := 500.0 // TODO(erh): this is proably the wrong default
		if v.Move.Speed != 0 {
			millisPerSec = v.Move.Speed
		}
		switch o := v.Move.Option.(type) {
		case *pb.MoveBase_StraightDistanceMillis:
			return &pb.ControlBaseResponse{}, base.MoveStraight(ctx, int(o.StraightDistanceMillis), millisPerSec, false)
		case *pb.MoveBase_SpinAngleDeg:
			return &pb.ControlBaseResponse{}, base.Spin(ctx, o.SpinAngleDeg, 64, false)
		default:
			return nil, fmt.Errorf("unknown move %T", o)
		}
	default:
		return nil, fmt.Errorf("unknown action %T", v)
	}
}

func (s *Server) MoveArmToPosition(ctx context.Context, req *pb.MoveArmToPositionRequest) (*pb.MoveArmToPositionResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.MoveArmToPositionResponse{}, arm.MoveToPosition(req.To)
}

func (s *Server) MoveArmToJointPositions(ctx context.Context, req *pb.MoveArmToJointPositionsRequest) (*pb.MoveArmToJointPositionsResponse, error) {
	arm := s.r.ArmByName(req.Name)
	if arm == nil {
		return nil, fmt.Errorf("no arm with name (%s)", req.Name)
	}

	return &pb.MoveArmToJointPositionsResponse{}, arm.MoveToJointPositions(req.To)
}

func (s *Server) ControlGripper(ctx context.Context, req *pb.ControlGripperRequest) (*pb.ControlGripperResponse, error) {
	gripper := s.r.GripperByName(req.Name)
	if gripper == nil {
		return nil, fmt.Errorf("no gripper with that name %s", req.Name)
	}

	var grabbed bool
	switch req.Action {
	case pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_OPEN:
		if err := gripper.Open(); err != nil {
			return nil, err
		}
	case pb.ControlGripperAction_CONTROL_GRIPPER_ACTION_GRAB:
		var err error
		grabbed, err = gripper.Grab()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unknown action: (%s)", req.Action)
	}

	return &pb.ControlGripperResponse{Grabbed: grabbed}, nil
}

func (s *Server) ControlBoardMotor(ctx context.Context, req *pb.ControlBoardMotorRequest) (*pb.ControlBoardMotorResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theMotor := b.Motor(req.MotorName)
	if theMotor == nil {
		return nil, fmt.Errorf("unknown motor: %s", req.MotorName)
	}

	// erh: this isn't right semantically.
	// GoFor with 0 rotations means something important.
	rVal := 0.0
	if req.Rotations != 0 {
		rVal = req.Rotations
	}

	if rVal == 0 {
		return &pb.ControlBoardMotorResponse{}, theMotor.Go(req.Direction, byte(req.Speed))
	}

	return &pb.ControlBoardMotorResponse{}, theMotor.GoFor(req.Direction, req.Speed, rVal)
}

func (s *Server) ControlBoardServo(ctx context.Context, req *pb.ControlBoardServoRequest) (*pb.ControlBoardServoResponse, error) {
	b := s.r.BoardByName(req.BoardName)
	if b == nil {
		return nil, fmt.Errorf("no board with name (%s)", req.BoardName)
	}

	theServo := b.Servo(req.ServoName)
	if theServo == nil {
		return nil, fmt.Errorf("unknown servo: %s", req.ServoName)
	}

	return &pb.ControlBoardServoResponse{}, theServo.Move(uint8(req.AngleDeg))
}
