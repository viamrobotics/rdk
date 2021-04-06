package board

import (
	"context"
	"fmt"

	pb "go.viam.com/robotcore/proto/api/v1"
)

func CreateStatus(ctx context.Context, b Board) (*pb.BoardStatus, error) {
	s := &pb.BoardStatus{
		Motors:            map[string]*pb.MotorStatus{},
		Servos:            map[string]*pb.ServoStatus{},
		Analogs:           map[string]*pb.AnalogStatus{},
		DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{},
	}

	cfg, err := b.GetConfig(ctx)
	if err != nil {
		return nil, err
	}

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
		s.Motors[name] = &pb.MotorStatus{
			On:                isOn,
			Position:          position,
			PositionSupported: positionSupported,
		}
	}

	for _, c := range cfg.Servos {
		name := c.Name
		x := b.Servo(name)
		current, err := x.Current(ctx)
		if err != nil {
			return nil, err
		}
		s.Servos[name] = &pb.ServoStatus{
			Angle: uint32(current),
		}
	}

	for _, c := range cfg.Analogs {
		name := c.Name
		x := b.AnalogReader(name)
		val, err := x.Read(ctx)
		if err != nil {
			return s, fmt.Errorf("couldn't read analog (%s) : %s", name, err)
		}
		s.Analogs[name] = &pb.AnalogStatus{Value: int32(val)}
	}

	for _, c := range cfg.DigitalInterrupts {
		name := c.Name
		x := b.DigitalInterrupt(name)
		s.DigitalInterrupts[name] = &pb.DigitalInterruptStatus{Value: x.Value()}
	}

	return s, nil
}
