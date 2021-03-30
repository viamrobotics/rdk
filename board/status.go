package board

import (
	"fmt"

	pb "go.viam.com/robotcore/proto/api/v1"
)

func CreateStatus(b Board) (*pb.BoardStatus, error) {
	s := &pb.BoardStatus{
		Motors:            map[string]*pb.MotorStatus{},
		Servos:            map[string]*pb.ServoStatus{},
		Analogs:           map[string]*pb.AnalogStatus{},
		DigitalInterrupts: map[string]*pb.DigitalInterruptStatus{},
	}

	cfg := b.GetConfig()

	for _, c := range cfg.Motors {
		name := c.Name
		x := b.Motor(name)
		s.Motors[name] = &pb.MotorStatus{
			On:                x.IsOn(),
			Position:          x.Position(),
			PositionSupported: x.PositionSupported(),
		}
	}

	for _, c := range cfg.Servos {
		name := c.Name
		x := b.Servo(name)
		s.Servos[name] = &pb.ServoStatus{
			Angle: uint32(x.Current()),
		}
	}

	for _, c := range cfg.Analogs {
		name := c.Name
		x := b.AnalogReader(name)
		val, err := x.Read()
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
