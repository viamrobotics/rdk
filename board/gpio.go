package board

import (
	"fmt"

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

func (m *GPIOMotor) Force(force byte) error {
	return m.Board.PWMSet(m.PWM, force)
}

func (m *GPIOMotor) Go(d Direction, force byte) error {
	switch d {
	case DirNone:
		return m.Off()
	case DirForward:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, force),
			m.Board.GPIOSet(m.A, true),
			m.Board.GPIOSet(m.B, false),
		)
	case DirBackward:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, force),
			m.Board.GPIOSet(m.A, false),
			m.Board.GPIOSet(m.B, true),
		)
	}

	return fmt.Errorf("unknown direction %v", d)
}

func (m *GPIOMotor) GoFor(d Direction, rpm float64, rotations float64) error {
	return fmt.Errorf("not supported")
}

func (m *GPIOMotor) IsOn() bool {
	return m.on
}

func (m *GPIOMotor) Off() error {
	m.on = false
	return multierr.Combine(
		m.Board.GPIOSet(m.A, false),
		m.Board.GPIOSet(m.B, false),
	)
}
