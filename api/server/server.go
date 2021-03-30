package server

import (
	"context"
	"time"

	"go.viam.com/robotcore/api"
	"go.viam.com/robotcore/board"
	pb "go.viam.com/robotcore/proto/api/v1"
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
	return &pb.StatusResponse{Status: StatusToProto(status)}, nil
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
		if err := server.Send(&pb.StatusStreamResponse{Status: StatusToProto(status)}); err != nil {
			return err
		}
	}
}

func StatusToProto(status api.Status) *pb.Status {
	var pbStatus pb.Status
	if status.Arms != nil {
		pbStatus.Arms = map[string]*pb.ArmStatus{}
		for key, value := range status.Arms {
			pbStatus.Arms[key] = ArmStatusToProto(value)
		}
	}
	pbStatus.Bases = status.Bases
	pbStatus.Grippers = status.Grippers
	if status.Boards != nil {
		pbStatus.Boards = map[string]*pb.BoardStatus{}
		for key, value := range status.Boards {
			pbStatus.Boards[key] = BoardStatusToProto(value)
		}
	}
	return &pbStatus
}

func ArmStatusToProto(status api.ArmStatus) *pb.ArmStatus {
	return &pb.ArmStatus{
		GridPosition:   ArmPositionToProto(status.GridPosition),
		JointPositions: JointPositionsToProto(status.JointPositions),
	}
}

func ArmPositionToProto(status api.ArmPosition) *pb.ArmPosition {
	return &pb.ArmPosition{
		X:  status.X,
		Y:  status.Y,
		Z:  status.Z,
		RX: status.Rx,
		RY: status.Ry,
		RZ: status.Rz,
	}
}

func JointPositionsToProto(status api.JointPositions) *pb.JointPositions {
	return &pb.JointPositions{
		Degrees: status.Degrees,
	}
}

func BoardStatusToProto(status board.Status) *pb.BoardStatus {
	var pbStatus pb.BoardStatus
	if status.Motors != nil {
		pbStatus.Motors = map[string]*pb.MotorStatus{}
		for key, value := range status.Motors {
			pbStatus.Motors[key] = MotorStatusToProto(value)
		}
	}
	if status.Servos != nil {
		pbStatus.Servos = map[string]*pb.ServoStatus{}
		for key, value := range status.Servos {
			pbStatus.Servos[key] = ServoStatusToProto(value)
		}
	}
	if status.Analogs != nil {
		pbStatus.Analogs = map[string]*pb.AnalogStatus{}
		for key, value := range status.Analogs {
			pbStatus.Analogs[key] = AnalogStatusToProto(value)
		}
	}
	if status.DigitalInterrupts != nil {
		pbStatus.DigitalInterrupts = map[string]*pb.DigitalInterruptStatus{}
		for key, value := range status.DigitalInterrupts {
			pbStatus.DigitalInterrupts[key] = DigitalInterruptStatusToProto(value)
		}
	}
	return &pbStatus
}

func MotorStatusToProto(status board.MotorStatus) *pb.MotorStatus {
	return &pb.MotorStatus{
		On:                status.On,
		PositionSupported: status.PositionSupported,
		Position:          status.Position,
	}
}

func ServoStatusToProto(status board.ServoStatus) *pb.ServoStatus {
	return &pb.ServoStatus{
		Angle: uint32(status.Angle),
	}
}

func AnalogStatusToProto(status board.AnalogStatus) *pb.AnalogStatus {
	return &pb.AnalogStatus{
		Value: int32(status.Value),
	}
}

func DigitalInterruptStatusToProto(status board.DigitalInterruptStatus) *pb.DigitalInterruptStatus {
	return &pb.DigitalInterruptStatus{
		Value: status.Value,
	}
}
