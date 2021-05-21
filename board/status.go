package board

import (
	"context"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"
)

// CreateStatus constructs a new up to date status from the given board.
// The operation can take time and be expensive, so it can be cancelled by the
// given context.
func CreateStatus(ctx context.Context, b Board) (*pb.BoardStatus, error) {
	var status pb.BoardStatus

	if names := b.MotorNames(); len(names) != 0 {
		status.Motors = make(map[string]*pb.MotorStatus, len(names))
		for _, name := range names {
			x := b.Motor(name)
			isOn, err := x.IsOn(ctx)
			if err != nil {
				return nil, err
			}
			position, err := x.Position(ctx)
			if err != nil {
				return nil, err
			}
			positionSupported, err := x.PositionSupported(ctx)
			if err != nil {
				return nil, err
			}
			status.Motors[name] = &pb.MotorStatus{
				On:                isOn,
				Position:          position,
				PositionSupported: positionSupported,
			}
		}
	}

	if names := b.ServoNames(); len(names) != 0 {
		status.Servos = make(map[string]*pb.ServoStatus, len(names))
		for _, name := range names {
			x := b.Servo(name)
			current, err := x.Current(ctx)
			if err != nil {
				return nil, err
			}
			status.Servos[name] = &pb.ServoStatus{
				Angle: uint32(current),
			}
		}
	}

	if names := b.AnalogReaderNames(); len(names) != 0 {
		status.Analogs = make(map[string]*pb.AnalogStatus, len(names))
		for _, name := range names {
			x := b.AnalogReader(name)
			val, err := x.Read(ctx)
			if err != nil {
				return nil, errors.Errorf("couldn't read analog (%s) : %w", name, err)
			}
			status.Analogs[name] = &pb.AnalogStatus{Value: int32(val)}
		}
	}

	if names := b.DigitalInterruptNames(); len(names) != 0 {
		status.DigitalInterrupts = make(map[string]*pb.DigitalInterruptStatus, len(names))
		for _, name := range names {
			x := b.DigitalInterrupt(name)
			status.DigitalInterrupts[name] = &pb.DigitalInterruptStatus{Value: x.Value()}
		}
	}

	return &status, nil
}
