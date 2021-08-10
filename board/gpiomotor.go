package board

import (
	"context"

	"github.com/go-errors/errors"

	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// NewGPIOMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewGPIOMotor(b Board, mc MotorConfig, logger golog.Logger) (Motor, error) {
	var m Motor
	pins := mc.Pins

	// If pins["c"] exists, then we have at least 3 data pins, and this is likely a stepper motor
	if _, ok := pins["c"]; ok {
		return NewGPIOStepperMotor(b, pins, mc, logger)
	}
	m = &GPIOMotor{
		b,
		pins["a"],
		pins["b"],
		pins["dir"],
		pins["pwm"],
		pins["en"],
		false,
		mc.PWMFreq,
	}
	return m, nil
}

var _ = Motor(&GPIOMotor{})

// A GPIOMotor is a GPIO based Motor that resides on a GPIO Board.
type GPIOMotor struct {
	Board              Board
	A, B, Dir, PWM, En string
	on                 bool
	pwmFreq            uint
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
	var errs error
	if powerPct > 0.0 && m.En != "" {
		errs = m.Board.GPIOSet(m.En, false)
	}
	return multierr.Combine(
		errs,
		m.Board.PWMSetFreq(m.PWM, m.pwmFreq),
		m.Board.PWMSet(m.PWM, byte(utils.ScaleByPct(255, float64(powerPct)))),
	)
}

// Go instructs the motor to operate at a certain power percentage in a given direction.
func (m *GPIOMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		m.on = true
		if m.Dir != "" {
			return multierr.Combine(
				m.Power(ctx, powerPct),
				m.Board.GPIOSet(m.Dir, true),
			)
		}
		return multierr.Combine(
			m.Power(ctx, powerPct),
			m.Board.GPIOSet(m.A, true),
			m.Board.GPIOSet(m.B, false),
		)
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		m.on = true
		if m.Dir != "" {
			return multierr.Combine(
				m.Power(ctx, powerPct),
				m.Board.GPIOSet(m.Dir, false),
			)
		}
		return multierr.Combine(
			m.Power(ctx, powerPct),
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
	var errs error
	if m.En != "" {
		errs = m.Board.GPIOSet(m.En, true)
	}
	if m.Dir == "" {
		errs = multierr.Combine(
			errs,
			m.Board.GPIOSet(m.A, false),
			m.Board.GPIOSet(m.B, false),
		)
	}
	return multierr.Combine(
		errs,
		m.Power(ctx, 0),
	)
}

// GoTo is not supported
func (m *GPIOMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	return errors.New("not supported")
}

// GoTillStop is not supported
func (m *GPIOMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64) error {
	return errors.New("not supported")
}

// Zero is not supported
func (m *GPIOMotor) Zero(ctx context.Context, offset float64) error {
	return errors.New("not supported")
}
