// Package board contains a gRPC based Board service server.
package board

import (
	"context"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/component/board/v1"

	"go.viam.com/rdk/protoutils"
	"go.viam.com/rdk/resource"
)

var (
	// ErrGPIOPinByNameReturnNil is the error returned when a gpio pin is nil.
	ErrGPIOPinByNameReturnNil = func(boardName string) error {
		return fmt.Errorf("board component %v GPIOPinByName should not return nil pin", boardName)
	}
	// ErrAnalogByNameReturnNil is the error returned when an analog is nil.
	ErrAnalogByNameReturnNil = func(boardName string) error {
		return fmt.Errorf("board component %v AnalogByName should not return nil analog", boardName)
	}
	// ErrDigitalInterruptByNameReturnNil is the error returned when a digital interrupt is nil.
	ErrDigitalInterruptByNameReturnNil = func(boardName string) error {
		return fmt.Errorf("board component %v DigitalInterruptByName should not return nil digital interrupt", boardName)
	}
)

// serviceServer implements the BoardService from board.proto.
type serviceServer struct {
	pb.UnimplementedBoardServiceServer
	coll resource.APIResourceCollection[Board]
}

// NewRPCServiceServer constructs an board gRPC service server.
// It is intentionally untyped to prevent use outside of tests.
func NewRPCServiceServer(coll resource.APIResourceCollection[Board]) interface{} {
	return &serviceServer{coll: coll}
}

// SetGPIO sets a given pin of a board of the underlying robot to either low or high.
func (s *serviceServer) SetGPIO(ctx context.Context, req *pb.SetGPIORequest) (*pb.SetGPIOResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	return &pb.SetGPIOResponse{}, p.Set(ctx, req.High, req.Extra.AsMap())
}

// GetGPIO gets the high/low state of a given pin of a board of the underlying robot.
func (s *serviceServer) GetGPIO(ctx context.Context, req *pb.GetGPIORequest) (*pb.GetGPIOResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	high, err := p.Get(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetGPIOResponse{High: high}, nil
}

// PWM gets the duty cycle of the given pin of a board of the underlying robot.
func (s *serviceServer) PWM(ctx context.Context, req *pb.PWMRequest) (*pb.PWMResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	pwm, err := p.PWM(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.PWMResponse{DutyCyclePct: pwm}, nil
}

// SetPWM sets a given pin of the underlying robot to the given duty cycle.
func (s *serviceServer) SetPWM(ctx context.Context, req *pb.SetPWMRequest) (*pb.SetPWMResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	return &pb.SetPWMResponse{}, p.SetPWM(ctx, req.DutyCyclePct, req.Extra.AsMap())
}

// PWMFrequency gets the PWM frequency of the given pin of a board of the underlying robot.
func (s *serviceServer) PWMFrequency(ctx context.Context, req *pb.PWMFrequencyRequest) (*pb.PWMFrequencyResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	freq, err := p.PWMFreq(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.PWMFrequencyResponse{FrequencyHz: uint64(freq)}, nil
}

// SetPWMFrequency sets a given pin of a board of the underlying robot to the given PWM frequency.
// For Raspberry Pis, 0 will use a default PWM frequency of 800.
func (s *serviceServer) SetPWMFrequency(
	ctx context.Context,
	req *pb.SetPWMFrequencyRequest,
) (*pb.SetPWMFrequencyResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	p, err := b.GPIOPinByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, ErrGPIOPinByNameReturnNil(req.Name)
	}

	return &pb.SetPWMFrequencyResponse{}, p.SetPWMFreq(ctx, uint(req.FrequencyHz), req.Extra.AsMap())
}

// ReadAnalogReader reads off the current value of an analog reader of a board of the underlying robot.
func (s *serviceServer) ReadAnalogReader(
	ctx context.Context,
	req *pb.ReadAnalogReaderRequest,
) (*pb.ReadAnalogReaderResponse, error) {
	b, err := s.coll.Resource(req.BoardName)
	if err != nil {
		return nil, err
	}

	theReader, err := b.AnalogByName(req.AnalogReaderName)
	if err != nil {
		return nil, err
	}
	if theReader == nil {
		return nil, ErrAnalogByNameReturnNil(req.BoardName)
	}

	analogValue, err := theReader.Read(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.ReadAnalogReaderResponse{
		Value:    int32(analogValue.Value),
		MinRange: analogValue.Min,
		MaxRange: analogValue.Max,
		StepSize: analogValue.StepSize,
	}, nil
}

// WriteAnalog writes the analog value to the analog writer pin of the underlying robot.
func (s *serviceServer) WriteAnalog(
	ctx context.Context,
	req *pb.WriteAnalogRequest,
) (*pb.WriteAnalogResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	analog, err := b.AnalogByName(req.Pin)
	if err != nil {
		return nil, err
	}
	if analog == nil {
		return nil, ErrAnalogByNameReturnNil(req.Name)
	}

	err = analog.Write(ctx, int(req.Value), req.Extra.AsMap())
	if err != nil {
		return nil, err
	}

	return &pb.WriteAnalogResponse{}, nil
}

// GetDigitalInterruptValue returns the current value of the interrupt which is based on the type of interrupt.
func (s *serviceServer) GetDigitalInterruptValue(
	ctx context.Context,
	req *pb.GetDigitalInterruptValueRequest,
) (*pb.GetDigitalInterruptValueResponse, error) {
	b, err := s.coll.Resource(req.BoardName)
	if err != nil {
		return nil, err
	}

	interrupt, err := b.DigitalInterruptByName(req.DigitalInterruptName)
	if err != nil {
		return nil, err
	}
	if interrupt == nil {
		return nil, ErrDigitalInterruptByNameReturnNil(req.BoardName)
	}

	val, err := interrupt.Value(ctx, req.Extra.AsMap())
	if err != nil {
		return nil, err
	}
	return &pb.GetDigitalInterruptValueResponse{Value: val}, nil
}

func (s *serviceServer) StreamTicks(
	req *pb.StreamTicksRequest,
	server pb.BoardService_StreamTicksServer,
) error {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return err
	}

	ticksChan := make(chan Tick)
	interrupts := []DigitalInterrupt{}

	for _, name := range req.PinNames {
		di, err := b.DigitalInterruptByName(name)
		if err != nil {
			return err
		}

		interrupts = append(interrupts, di)
	}
	err = b.StreamTicks(server.Context(), interrupts, ticksChan, req.Extra.AsMap())
	if err != nil {
		return err
	}

	// Send an empty response first so the client doesn't block while checking for errors.
	err = server.Send(&pb.StreamTicksResponse{})
	if err != nil {
		return err
	}

	for {
		select {
		case <-server.Context().Done():
			return server.Context().Err()
		default:
		}

		select {
		case <-server.Context().Done():
			return server.Context().Err()
		case msg := <-ticksChan:
			err := server.Send(&pb.StreamTicksResponse{PinName: msg.Name, High: msg.High, Time: msg.TimestampNanosec})
			if err != nil {
				return err
			}
		}
	}
}

// DoCommand receives arbitrary commands.
func (s *serviceServer) DoCommand(ctx context.Context,
	req *commonpb.DoCommandRequest,
) (*commonpb.DoCommandResponse, error) {
	b, err := s.coll.Resource(req.GetName())
	if err != nil {
		return nil, err
	}
	return protoutils.DoFromResourceServer(ctx, b, req)
}

func (s *serviceServer) SetPowerMode(ctx context.Context,
	req *pb.SetPowerModeRequest,
) (*pb.SetPowerModeResponse, error) {
	b, err := s.coll.Resource(req.Name)
	if err != nil {
		return nil, err
	}

	if req.Duration == nil {
		err = b.SetPowerMode(ctx, req.PowerMode, nil)
	} else {
		if err := req.Duration.CheckValid(); err != nil {
			return nil, err
		}
		duration := req.Duration.AsDuration()
		err = b.SetPowerMode(ctx, req.PowerMode, &duration)
	}

	if err != nil {
		return nil, err
	}

	return &pb.SetPowerModeResponse{}, nil
}
