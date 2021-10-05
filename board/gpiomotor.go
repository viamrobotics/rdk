package board

import (
	"context"

	"github.com/go-errors/errors"

	"go.viam.com/core/motor"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.uber.org/multierr"
)

// NewGPIOMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewGPIOMotor(b Board, mc motor.Config, logger golog.Logger) (motor.Motor, error) {
	var m motor.Motor
	pins := mc.Pins

	if mc.MaxPowerPct == 0 {
		mc.MaxPowerPct = 1.0
	}
	if mc.MaxPowerPct < 0.06 || mc.MaxPowerPct > 1.0 {
		return nil, errors.New("max_power_pct must be between 0.06 and 1.0")
	}

	if mc.MinPowerPct < 0 {
		mc.MinPowerPct = 0
	} else if mc.MinPowerPct > 1.0 {
		mc.MinPowerPct = 1.0
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
		pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED,
		mc.MinPowerPct,
		mc.MaxPowerPct,
	}
	return m, nil
}

var _ = motor.Motor(&GPIOMotor{})

// A GPIOMotor is a GPIO based Motor that resides on a GPIO Board.
type GPIOMotor struct {
	Board              Board
	A, B, Dir, PWM, En string
	on                 bool
	pwmFreq            uint
	curDirection       pb.DirectionRelative
	minPowerPct        float32
	maxPowerPct        float32
}

// Position always returns 0.
func (m *GPIOMotor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

// PositionSupported always returns false.
func (m *GPIOMotor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

// Power sets the associated pins (as discovered) and sets PWM to the given power percentage.
func (m *GPIOMotor) Power(ctx context.Context, powerPct float32) error {
	var errs error
	if powerPct > m.maxPowerPct {
		powerPct = m.maxPowerPct
	}

	if powerPct <= 0.001 {
		if m.En != "" {
			errs = m.Board.GPIOSet(ctx, m.En, true)
		}

		if m.A != "" && m.B != "" {
			errs = multierr.Combine(
				errs,
				m.Board.GPIOSet(ctx, m.A, false),
				m.Board.GPIOSet(ctx, m.B, false),
			)
		}

		if m.PWM != "" {
			errs = multierr.Combine(errs, m.Board.GPIOSet(ctx, m.PWM, false))
		}
		return errs
	}

	m.on = true
	if m.En != "" {
		errs = multierr.Combine(errs, m.Board.GPIOSet(ctx, m.En, false))
	}

	var pwmPin string
	if m.PWM != "" {
		pwmPin = m.PWM
	} else if m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		pwmPin = m.B
		powerPct = 1.0 - powerPct // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	} else if m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		pwmPin = m.A
		powerPct = 1.0 - powerPct // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	} else if m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return errors.New("can't set power when no direction is set")
	}

	if powerPct < m.minPowerPct {
		powerPct = m.minPowerPct
	}

	return multierr.Combine(
		errs,
		m.Board.PWMSetFreq(ctx, pwmPin, m.pwmFreq),
		m.Board.PWMSet(ctx, pwmPin, byte(utils.ScaleByPct(255, float64(powerPct)))),
	)
}

// Go instructs the motor to operate at a certain power percentage in a given direction.
func (m *GPIOMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	switch d {
	case pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED:
		return m.Off(ctx)
	case pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD:
		m.curDirection = pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
		if m.Dir != "" {
			return multierr.Combine(
				m.Board.GPIOSet(ctx, m.Dir, true),
				m.Power(ctx, powerPct),
			)
		}
		return multierr.Combine(
			m.Board.GPIOSet(ctx, m.A, true),
			m.Board.GPIOSet(ctx, m.B, false),
			m.Power(ctx, powerPct), // Must be last for A/B only drivers
		)
	case pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD:
		m.curDirection = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		if m.Dir != "" {
			return multierr.Combine(
				m.Board.GPIOSet(ctx, m.Dir, false),
				m.Power(ctx, powerPct),
			)
		}
		return multierr.Combine(
			m.Board.GPIOSet(ctx, m.A, false),
			m.Board.GPIOSet(ctx, m.B, true),
			m.Power(ctx, powerPct), // Must be last for A/B only motors (where PWM will take over one of A or B)
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
	m.curDirection = pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED
	return m.Power(ctx, 0)
}

// GoTo is not supported
func (m *GPIOMotor) GoTo(ctx context.Context, rpm float64, position float64) error {
	return errors.New("not supported")
}

// GoTillStop is not supported
func (m *GPIOMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// Zero is not supported
func (m *GPIOMotor) Zero(ctx context.Context, offset float64) error {
	return errors.New("not supported")
}
