// Package board contains a gRPC based Board service server.
package board

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the contract from board_subtype.proto.
type subtypeServer struct {
	pb.UnimplementedBoardServiceServer
	s subtype.Service
}

// NewServer constructs an board gRPC service server.
func NewServer(s subtype.Service) pb.BoardServiceServer {
	return &subtypeServer{s: s}
}

// getBoard returns the board specified, nil if not.
func (s *subtypeServer) getBoard(name string) (Board, error) {
	resource := s.s.Resource(name)
	if resource == nil {
		return nil, errors.Errorf("no board with name (%s)", name)
	}
	board, ok := resource.(Board)
	if !ok {
		return nil, errors.Errorf("resource with name (%s) is not a board", name)
	}
	return board, nil
}

// Status returns the status of a board of the underlying robot.
func (s *subtypeServer) Status(ctx context.Context, req *pb.BoardServiceStatusRequest) (*pb.BoardServiceStatusResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	status, err := b.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceStatusResponse{Status: status}, nil
}

// GPIOSet sets a given pin of a board of the underlying robot to either low or high.
func (s *subtypeServer) GPIOSet(ctx context.Context, req *pb.BoardServiceGPIOSetRequest) (*pb.BoardServiceGPIOSetResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServiceGPIOSetResponse{}, b.GPIOSet(ctx, req.Pin, req.High)
}

// GPIOGet gets the high/low state of a given pin of a board of the underlying robot.
func (s *subtypeServer) GPIOGet(ctx context.Context, req *pb.BoardServiceGPIOGetRequest) (*pb.BoardServiceGPIOGetResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	high, err := b.GPIOGet(ctx, req.Pin)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceGPIOGetResponse{High: high}, nil
}

// PWMSet sets a given pin of the underlying robot to the given duty cycle.
func (s *subtypeServer) PWMSet(ctx context.Context, req *pb.BoardServicePWMSetRequest) (*pb.BoardServicePWMSetResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServicePWMSetResponse{}, b.PWMSet(ctx, req.Pin, byte(req.DutyCycle))
}

// PWMSetFrequency sets a given pin of a board of the underlying robot to the given PWM frequency. 0 will use the board's default PWM
// frequency.
func (s *subtypeServer) PWMSetFrequency(
	ctx context.Context,
	req *pb.BoardServicePWMSetFrequencyRequest,
) (*pb.BoardServicePWMSetFrequencyResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.BoardServicePWMSetFrequencyResponse{}, b.PWMSetFreq(ctx, req.Pin, uint(req.Frequency))
}

// AnalogReaderRead reads off the current value of an analog reader of a board of the underlying robot.
func (s *subtypeServer) AnalogReaderRead(
	ctx context.Context,
	req *pb.BoardServiceAnalogReaderReadRequest,
) (*pb.BoardServiceAnalogReaderReadResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	theReader, ok := b.AnalogReaderByName(req.AnalogReaderName)
	if !ok {
		return nil, errors.Errorf("unknown analog reader: %s", req.AnalogReaderName)
	}

	val, err := theReader.Read(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceAnalogReaderReadResponse{Value: int32(val)}, nil
}

// DigitalInterruptConfig returns the config the interrupt was created with.
func (s *subtypeServer) DigitalInterruptConfig(
	ctx context.Context,
	req *pb.BoardServiceDigitalInterruptConfigRequest,
) (*pb.BoardServiceDigitalInterruptConfigResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	config, err := interrupt.Config(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceDigitalInterruptConfigResponse{Config: digitalInterruptConfigToProto(&config)}, nil
}

func digitalInterruptConfigToProto(config *DigitalInterruptConfig) *pb.DigitalInterruptConfig {
	return &pb.DigitalInterruptConfig{
		Name:    config.Name,
		Pin:     config.Pin,
		Type:    config.Type,
		Formula: config.Formula,
	}
}

// DigitalInterruptValue returns the current value of the interrupt which is based on the type of interrupt.
func (s *subtypeServer) DigitalInterruptValue(
	ctx context.Context,
	req *pb.BoardServiceDigitalInterruptValueRequest,
) (*pb.BoardServiceDigitalInterruptValueResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	val, err := interrupt.Value(ctx)
	if err != nil {
		return nil, err
	}
	return &pb.BoardServiceDigitalInterruptValueResponse{Value: val}, nil
}

// DigitalInterruptTick is to be called either manually if the interrupt is a proxy to some real hardware interrupt or for tests.
func (s *subtypeServer) DigitalInterruptTick(
	ctx context.Context,
	req *pb.BoardServiceDigitalInterruptTickRequest,
) (*pb.BoardServiceDigitalInterruptTickResponse, error) {
	b, err := s.getBoard(req.BoardName)
	if err != nil {
		return nil, err
	}

	interrupt, ok := b.DigitalInterruptByName(req.DigitalInterruptName)
	if !ok {
		return nil, errors.Errorf("unknown digital interrupt: %s", req.DigitalInterruptName)
	}

	return &pb.BoardServiceDigitalInterruptTickResponse{}, interrupt.Tick(ctx, req.High, req.Nanos)
}
