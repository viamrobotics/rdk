package board

import (
	"context"

	"github.com/go-errors/errors"

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

	// PWMSetFreq sets the given pin to the given PWM frequency. 0 will use the board's default PWM frequency.
	PWMSetFreq(pin string, freq uint) error
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
		mc.PWMFreq,
	}
	return m, nil
}

var _ = Motor(&GPIOMotor{})

// A GPIOMotor is a GPIO based Motor that resides on a GPIO Board.
type GPIOMotor struct {
	Board     GPIOBoard
	A, B, PWM string
	on        bool
	pwmFreq   uint
}

// Position always returns 0.
func (m *GPIOMotor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

// PositionSupported always returns false.
func (m *GPIOMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

// Power sets the associated pins PWM to the given power percentage.
func (m *GPIOMotor) Power(ctx context.Context, powerPct float32) error {
	err := m.Board.PWMSetFreq(m.PWM, m.pwmFreq)
	if err != nil {
		return err
	}
	return m.Board.PWMSet(m.PWM, byte(utils.ScaleByPct(255, float64(powerPct))))
}

// Go instructs the motor to operate at a certain power percentage in a given direction.
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

	return errors.Errorf("unknown direction %v", d)
}

// GoFor is not yet supported.
func (m *GPIOMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	return errors.New("not supported")
}

// IsOn returns if the motor is currently on or off.
func (m *GPIOMotor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Off turns the motor off by setting the appropriate pins to low states.
func (m *GPIOMotor) Off(ctx context.Context) error {
	m.on = false
	return multierr.Combine(
		m.Board.GPIOSet(m.A, false),
		m.Board.GPIOSet(m.B, false),
	)
}
