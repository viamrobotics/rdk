package gpio

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/operation"
)

// NewMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewMotor(b board.Board, mc Config, name string, logger golog.Logger) (motor.Motor, error) {
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

	m := &Motor{
		Board:       b,
		on:          false,
		pwmFreq:     mc.PWMFreq,
		minPowerPct: mc.MinPowerPct,
		maxPowerPct: mc.MaxPowerPct,
		maxRPM:      mc.MaxRPM,
		dirFlip:     mc.DirectionFlip,
		logger:      logger,
		motorName:   name,
	}

	if mc.Pins.A != "" {
		a, err := b.GPIOPinByName(mc.Pins.A)
		if err != nil {
			return nil, err
		}
		m.A = a
	}
	if mc.Pins.B != "" {
		b, err := b.GPIOPinByName(mc.Pins.B)
		if err != nil {
			return nil, err
		}
		m.B = b
	}
	if mc.Pins.Direction != "" {
		direction, err := b.GPIOPinByName(mc.Pins.Direction)
		if err != nil {
			return nil, err
		}
		m.Direction = direction
	}
	if mc.Pins.PWM != "" {
		pwm, err := b.GPIOPinByName(mc.Pins.PWM)
		if err != nil {
			return nil, err
		}
		m.PWM = pwm
	}
	if mc.Pins.EnablePinHigh != "" {
		enablePinHigh, err := b.GPIOPinByName(mc.Pins.EnablePinHigh)
		if err != nil {
			return nil, err
		}
		m.EnablePinHigh = enablePinHigh
	}
	if mc.Pins.EnablePinLow != "" {
		enablePinLow, err := b.GPIOPinByName(mc.Pins.EnablePinLow)
		if err != nil {
			return nil, err
		}
		m.EnablePinLow = enablePinLow
	}

	return m, nil
}

var _ = motor.LocalMotor(&Motor{})

// A Motor is a GPIO based Motor that resides on a GPIO Board.
type Motor struct {
	mu     sync.Mutex
	opMgr  operation.SingleOperationManager
	logger golog.Logger
	// config
	Board                    board.Board
	A, B, Direction, PWM, En board.GPIOPin
	EnablePinLow             board.GPIOPin
	EnablePinHigh            board.GPIOPin
	pwmFreq                  uint
	minPowerPct              float64
	maxPowerPct              float64
	maxRPM                   float64
	dirFlip                  bool
	motorName                string
	// state
	on       bool
	powerPct float64

	generic.Unimplemented
}

// Position always returns 0.
func (m *Motor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	return 0, nil
}

// Properties returns the status of whether the motor supports certain optional features.
func (m *Motor) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: false,
	}, nil
}

// setPWM sets the associated pins (as discovered) and sets PWM to the given power percentage.
// Anything calling setPWM MUST lock the motor's mutex prior.
func (m *Motor) setPWM(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	var errs error
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)

	if math.Abs(powerPct) <= 0.001 {
		m.powerPct = 0.0
		m.on = false
		if m.EnablePinLow != nil {
			errs = multierr.Combine(errs, m.EnablePinLow.Set(ctx, true, extra))
		}
		if m.EnablePinHigh != nil {
			errs = multierr.Combine(errs, m.EnablePinHigh.Set(ctx, false, extra))
		}

		if m.A != nil && m.B != nil {
			errs = multierr.Combine(
				errs,
				m.A.Set(ctx, false, extra),
				m.B.Set(ctx, false, extra),
			)
		}

		if m.PWM != nil {
			errs = multierr.Combine(errs, m.PWM.Set(ctx, false, extra))
		}
		return errs
	}

	m.on = true
	if m.EnablePinLow != nil {
		errs = multierr.Combine(errs, m.EnablePinLow.Set(ctx, false, extra))
	}
	if m.EnablePinHigh != nil {
		errs = multierr.Combine(errs, m.EnablePinHigh.Set(ctx, true, extra))
	}

	var pwmPin board.GPIOPin
	switch {
	case m.PWM != nil:
		pwmPin = m.PWM
	case powerPct >= 0.001:
		pwmPin = m.B
		if m.dirFlip {
			pwmPin = m.A
		}
		powerPct = 1.0 - math.Abs(powerPct) // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	case powerPct <= -0.001:
		pwmPin = m.A
		if m.dirFlip {
			pwmPin = m.B
		}
		powerPct = 1.0 - math.Abs(powerPct) // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	default:
		return errors.New("can't set power when no direction is set")
	}

	powerPct = math.Max(math.Abs(powerPct), m.minPowerPct)
	m.powerPct = powerPct
	return multierr.Combine(
		errs,
		pwmPin.SetPWMFreq(ctx, m.pwmFreq, extra),
		pwmPin.SetPWM(ctx, powerPct, extra),
	)
}

// SetPower instructs the motor to operate at an rpm, where the sign of the rpm
// indicates direction.
func (m *Motor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	if math.Abs(powerPct) <= 0.01 {
		return m.Stop(ctx, extra)
	}
	// Stop locks/unlocks the mutex as well so in the case that the power ~= 0
	// we want to simply rely on the mutex use in Stop
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Direction != nil {
		x := !math.Signbit(powerPct)
		if m.dirFlip {
			x = !x
		}
		return multierr.Combine(
			m.Direction.Set(ctx, x, extra),
			m.setPWM(ctx, powerPct, extra),
		)
	}
	if m.A != nil && m.B != nil {
		a := m.A
		b := m.B
		if m.dirFlip {
			a = m.B
			b = m.A
		}
		return multierr.Combine(
			a.Set(ctx, !math.Signbit(powerPct), extra),
			b.Set(ctx, math.Signbit(powerPct), extra),
			m.setPWM(ctx, powerPct, extra), // Must be last for A/B only drivers
		)
	}

	if !math.Signbit(powerPct) {
		return m.setPWM(ctx, powerPct, extra)
	}

	return errors.New("trying to go backwards but don't have dir or a&b pins")
}

// If revolutions is 0, the returned wait duration will be 0 representing that
// the motor should run indefinitely.
func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	if revolutions == 0 {
		powerPct := rpm / maxRPM
		return powerPct, 0
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

// GoFor moves an inputted number of revolutions at the given rpm, no encoder is present
// for this so power is determined via a linear relationship with the maxRPM and the distance
// traveled is a time based estimation based on desired RPM.
func (m *Motor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if m.maxRPM == 0 {
		return errors.New("not supported, define max_rpm attribute != 0")
	}
	if rpm == 0 {
		return motor.NewZeroRPMError()
	}

	powerPct, waitDur := goForMath(m.maxRPM, rpm, revolutions)
	err := m.SetPower(ctx, powerPct, extra)
	if err != nil {
		return errors.Wrapf(err, "error in GoFor from motor (%s)", m.motorName)
	}

	if revolutions == 0 {
		return nil
	}

	if m.opMgr.NewTimedWaitOp(ctx, waitDur) {
		return m.Stop(ctx, extra)
	}
	return nil
}

// IsPowered returns if the motor is currently on or off.
func (m *Motor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.on, m.powerPct, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down, by setting the appropriate pins to low states.
func (m *Motor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setPWM(ctx, 0, extra)
}

// IsMoving returns if the motor is currently on or off.
func (m *Motor) IsMoving(ctx context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.on, nil
}

// GoTo is not supported.
func (m *Motor) GoTo(ctx context.Context, rpm, positionRevolutions float64, extra map[string]interface{}) error {
	return motor.NewGoToUnsupportedError(m.motorName)
}

// ResetZeroPosition is not supported.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return motor.NewResetZeroPositionUnsupportedError(m.motorName)
}

// GoTillStop is not supported.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return motor.NewGoTillStopUnsupportedError(m.motorName)
}
