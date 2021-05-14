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

	cfg, err := b.Config(ctx)
	if err != nil {
		return nil, err
	}

	if len(cfg.Motors) != 0 {
		status.Motors = make(map[string]*pb.MotorStatus, len(cfg.Motors))
		for _, c := range cfg.Motors {
			name := c.Name
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

	if len(cfg.Servos) != 0 {
		status.Servos = make(map[string]*pb.ServoStatus, len(cfg.Servos))
		for _, c := range cfg.Servos {
			name := c.Name
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

	if len(cfg.Analogs) != 0 {
		status.Analogs = make(map[string]*pb.AnalogStatus, len(cfg.Analogs))
		for _, c := range cfg.Analogs {
			name := c.Name
			x := b.AnalogReader(name)
			val, err := x.Read(ctx)
			if err != nil {
				return nil, errors.Errorf("couldn't read analog (%s) : %w", name, err)
			}
			status.Analogs[name] = &pb.AnalogStatus{Value: int32(val)}
		}
	}

	if len(cfg.DigitalInterrupts) != 0 {
		status.DigitalInterrupts = make(map[string]*pb.DigitalInterruptStatus, len(cfg.DigitalInterrupts))
		for _, c := range cfg.DigitalInterrupts {
			name := c.Name
			x := b.DigitalInterrupt(name)
			status.DigitalInterrupts[name] = &pb.DigitalInterruptStatus{Value: x.Value()}
		}
	}

	return &status, nil
}
