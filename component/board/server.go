// Package board contains a gRPC based Board service server.
package board

import (
	"context"

	"github.com/pkg/errors"

	pb "go.viam.com/rdk/proto/api/component/board/v1"
	"go.viam.com/rdk/subtype"
)

// subtypeServer implements the BoardService from board.proto.
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
func (s *subtypeServer) Status(ctx context.Context, req *pb.StatusRequest) (*pb.StatusResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	status, err := b.Status(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.StatusResponse{Status: status}, nil
}

// SetGPIO sets a given pin of a board of the underlying robot to either low or high.
func (s *subtypeServer) SetGPIO(ctx context.Context, req *pb.SetGPIORequest) (*pb.SetGPIOResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.SetGPIOResponse{}, b.SetGPIO(ctx, req.Pin, req.High)
}

// GetGPIO gets the high/low state of a given pin of a board of the underlying robot.
func (s *subtypeServer) GetGPIO(ctx context.Context, req *pb.GetGPIORequest) (*pb.GetGPIOResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	high, err := b.GetGPIO(ctx, req.Pin)
	if err != nil {
		return nil, err
	}
	return &pb.GetGPIOResponse{High: high}, nil
}

// SetPWM sets a given pin of the underlying robot to the given duty cycle.
func (s *subtypeServer) SetPWM(ctx context.Context, req *pb.SetPWMRequest) (*pb.SetPWMResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.SetPWMResponse{}, b.SetPWM(ctx, req.Pin, req.DutyCyclePct)
}

// SetPWMFrequency sets a given pin of a board of the underlying robot to the given PWM frequency. 0 will use the board's default PWM
// frequency.
func (s *subtypeServer) SetPWMFrequency(
	ctx context.Context,
	req *pb.SetPWMFrequencyRequest,
) (*pb.SetPWMFrequencyResponse, error) {
	b, err := s.getBoard(req.Name)
	if err != nil {
		return nil, err
	}

	return &pb.SetPWMFrequencyResponse{}, b.SetPWMFreq(ctx, req.Pin, uint(req.FrequencyHz))
}

// ReadAnalogReader reads off the current value of an analog reader of a board of the underlying robot.
func (s *subtypeServer) ReadAnalogReader(
	ctx context.Context,
	req *pb.ReadAnalogReaderRequest,
) (*pb.ReadAnalogReaderResponse, error) {
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
	return &pb.ReadAnalogReaderResponse{Value: int32(val)}, nil
}

// GetDigitalInterruptValue returns the current value of the interrupt which is based on the type of interrupt.
func (s *subtypeServer) GetDigitalInterruptValue(
	ctx context.Context,
	req *pb.GetDigitalInterruptValueRequest,
) (*pb.GetDigitalInterruptValueResponse, error) {
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
	return &pb.GetDigitalInterruptValueResponse{Value: val}, nil
}
