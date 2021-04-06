package board

import (
	"context"
	"fmt"

	pb "go.viam.com/robotcore/proto/api/v1"

	"go.uber.org/multierr"
)

type GPIOBoard interface {
	GPIOSet(pin string, high bool) error
	PWMSet(pin string, dutyCycle byte) error
}

func NewGPIOMotor(b GPIOBoard, pins map[string]string) (*GPIOMotor, error) {
	m := &GPIOMotor{
		b,
		pins["a"],
		pins["b"],
		pins["pwm"],
		false,
	}
	return m, nil
}

type GPIOMotor struct {
	Board     GPIOBoard
	A, B, PWM string
	on        bool
}

func (m *GPIOMotor) Position(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *GPIOMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *GPIOMotor) Force(ctx context.Context, force byte) error {
	return m.Board.PWMSet(m.PWM, force)
}

func (m *GPIOMotor) Go(ctx context.Context, d pb.DirectionRelative, force byte) error {
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, force),
			m.Board.GPIOSet(m.A, true),
			m.Board.GPIOSet(m.B, false),
		)
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, force),
			m.Board.GPIOSet(m.A, false),
			m.Board.GPIOSet(m.B, true),
		)
	}

	return fmt.Errorf("unknown direction %v", d)
}

func (m *GPIOMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, rotations float64) error {
	return fmt.Errorf("not supported")
}

func (m *GPIOMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

func (m *GPIOMotor) Off(ctx context.Context) error {
	m.on = false
	return multierr.Combine(
		m.Board.GPIOSet(m.A, false),
		m.Board.GPIOSet(m.B, false),
	)
}
