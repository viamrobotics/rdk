package board

import (
	"context"
	"errors"
	"fmt"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// A GPIOBoard is a board that allows for setting attributes on specific pins.
type GPIOBoard interface {
	// GPIOSet sets the given pin to either low or high.
	GPIOSet(pin string, high bool) error

	// PWMSet sets the given pin to the given duty cycle.
	PWMSet(pin string, dutyCycle byte) error
}

// NewGPIOMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewGPIOMotor(b GPIOBoard, mc MotorConfig, logger golog.Logger) (Motor, error) {
	var m Motor
	pins := mc.Pins

	// If pins["c"] exists, then we have at least 3 data pins, and this is likely a stepper motor
	if _, ok := pins["c"]; ok {
		return NewBrushlessMotor(b, pins, mc, logger)
	}
	m = &GPIOMotor{
		b,
		pins["a"],
		pins["b"],
		pins["pwm"],
		false,
	}
	return m, nil
}

var _ = Motor(&GPIOMotor{})

// A GPIOMotor is a GPIO based Motor that resides ona GPIO Board.
type GPIOMotor struct {
	Board     GPIOBoard
	A, B, PWM string
	on        bool
}

func (m *GPIOMotor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

func (m *GPIOMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

func (m *GPIOMotor) Power(ctx context.Context, powerPct float32) error {
	return m.Board.PWMSet(m.PWM, byte(utils.ScaleByPct(255, float64(powerPct))))
}

func (m *GPIOMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	power := byte(utils.ScaleByPct(255, float64(powerPct)))
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, power),
			m.Board.GPIOSet(m.A, true),
			m.Board.GPIOSet(m.B, false),
		)
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		m.on = true
		return multierr.Combine(
			m.Board.PWMSet(m.PWM, power),
			m.Board.GPIOSet(m.A, false),
			m.Board.GPIOSet(m.B, true),
		)
	}

	return fmt.Errorf("unknown direction %v", d)
}

func (m *GPIOMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	return errors.New("not supported")
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
