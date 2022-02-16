package gpio

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/motor"
)

// NewMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewMotor(b board.Board, mc motor.Config, logger golog.Logger) (motor.Motor, error) {
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
		Board:         b,
		A:             mc.Pins.A,
		B:             mc.Pins.B,
		Dir:           mc.Pins.Dir,
		PWM:           mc.Pins.PWM,
		EnablePinHigh: mc.Pins.EnablePinHigh,
		EnablePinLow:  mc.Pins.EnablePinLow,
		En:            mc.Pins.En,
		on:            false,
		pwmFreq:       mc.PWMFreq,
		minPowerPct:   mc.MinPowerPct,
		maxPowerPct:   mc.MaxPowerPct,
		maxRPM:        mc.MaxRPM,
		dirFlip:       mc.DirFlip,
		logger:        logger,
		cancelMu:      &sync.Mutex{},
	}

	if m.EnablePinLow == "" {
		// for backwards compatibility prior ot change on 1/21/22
		m.EnablePinLow = mc.Pins.En
	}

	return m, nil
}

var _ = motor.Motor(&Motor{})

// A Motor is a GPIO based Motor that resides on a GPIO Board.
type Motor struct {
	Board              board.Board
	A, B, Dir, PWM, En string
	EnablePinLow       string
	EnablePinHigh      string
	on                 bool
	pwmFreq            uint
	minPowerPct        float64
	maxPowerPct        float64
	maxRPM             float64
	dirFlip            bool

	cancelMu      *sync.Mutex
	cancelForFunc func()
	waitCh        chan struct{}
	logger        golog.Logger
}

// GetPosition always returns 0.
func (m *Motor) GetPosition(ctx context.Context) (float64, error) {
	return 0, nil
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *Motor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: false,
	}, nil
}

// setPWM sets the associated pins (as discovered) and sets PWM to the given power percentage.
func (m *Motor) setPWM(ctx context.Context, powerPct float64) error {
	var errs error
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)

	if math.Abs(powerPct) <= 0.001 {
		if m.EnablePinLow != "" {
			errs = m.Board.SetGPIO(ctx, m.EnablePinLow, true)
		}
		if m.EnablePinHigh != "" {
			errs = m.Board.SetGPIO(ctx, m.EnablePinHigh, false)
		}

		if m.A != "" && m.B != "" {
			errs = multierr.Combine(
				errs,
				m.Board.SetGPIO(ctx, m.A, false),
				m.Board.SetGPIO(ctx, m.B, false),
			)
		}

		if m.PWM != "" {
			errs = multierr.Combine(errs, m.Board.SetGPIO(ctx, m.PWM, false))
		}
		return errs
	}

	m.on = true
	if m.EnablePinLow != "" {
		errs = multierr.Combine(errs, m.Board.SetGPIO(ctx, m.EnablePinLow, false))
	}
	if m.EnablePinHigh != "" {
		errs = multierr.Combine(errs, m.Board.SetGPIO(ctx, m.EnablePinHigh, true))
	}

	var pwmPin string
	switch {
	case m.PWM != "":
		pwmPin = m.PWM
	case powerPct >= 0.001:
		pwmPin = m.B
		powerPct = 1.0 - math.Abs(powerPct) // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	case powerPct <= -0.001:
		pwmPin = m.A
		powerPct = 1.0 - math.Abs(powerPct) // Other pin is always high, so only when PWM is LOW are we driving. Thus, we invert here.
	default:
		return errors.New("can't set power when no direction is set")
	}

	powerPct = math.Max(math.Abs(powerPct), m.minPowerPct)
	return multierr.Combine(
		errs,
		m.Board.SetPWMFreq(ctx, pwmPin, m.pwmFreq),
		m.Board.SetPWM(ctx, pwmPin, powerPct),
	)
}

//  SetPower instructs the motor to operate at an rpm, where the sign of the rpm
// indicates direction.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	m.cancelMu.Lock()
	m.cancelWaitProcesses()
	m.cancelMu.Unlock()

	if math.Abs(powerPct) <= 0.01 {
		return m.Stop(ctx)
	}

	if m.Dir != "" {
		x := !math.Signbit(powerPct)
		if m.dirFlip {
			x = !x
		}
		return multierr.Combine(
			m.Board.SetGPIO(ctx, m.Dir, x),
			m.setPWM(ctx, powerPct),
		)
	}
	if m.A != "" && m.B != "" {
		return multierr.Combine(
			m.Board.SetGPIO(ctx, m.A, !math.Signbit(powerPct)),
			m.Board.SetGPIO(ctx, m.B, math.Signbit(powerPct)),
			m.setPWM(ctx, powerPct), // Must be last for A/B only drivers
		)
	}

	if !math.Signbit(powerPct) {
		return m.setPWM(ctx, powerPct)
	}

	return errors.New("trying to go backwards but don't have dir or a&b pins")
}

func goForMath(maxRPM, rpm, revolutions float64) (float64, time.Duration) {
	// need to do this so time is reasonable
	if rpm > maxRPM {
		rpm = maxRPM
	} else if rpm < -1*maxRPM {
		rpm = -1 * maxRPM
	}

	dir := rpm * revolutions / math.Abs(revolutions*rpm)
	powerPct := math.Abs(rpm) / maxRPM * dir
	waitDur := time.Duration(math.Abs(revolutions/rpm)*60*1000) * time.Millisecond
	return powerPct, waitDur
}

// GoFor moves an inputted number of revolutions at the given rpm, no encoder is present
// for this so power is deteremiend via a linear relationship with the maxRPM and the distance
// traveled is a time based estimation based on desired RPM.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	if m.maxRPM == 0 {
		return errors.New("not supported, define max_rpm attribute")
	}

	powerPct, waitDur := goForMath(m.maxRPM, rpm, revolutions)
	err := m.SetPower(ctx, powerPct)
	if err != nil {
		return err
	}

	// Begin go process to track timing and turn off motors after estimated distances has been traveled
	ctxWithTimeout, cancelForFunc := context.WithTimeout(context.Background(), waitDur)
	waitCh := make(chan struct{})

	m.cancelMu.Lock()
	m.cancelWaitProcesses()
	m.waitCh = waitCh
	m.cancelForFunc = cancelForFunc
	m.cancelMu.Unlock()

	goutils.PanicCapturingGo(func() {
		defer close(m.waitCh)
		<-ctxWithTimeout.Done()
		if errors.Is(ctxWithTimeout.Err(), context.DeadlineExceeded) {
			// this has to be new context as previous one is likely timedout
			//nolint:contextcheck
			err := m.Stop(context.Background())
			if err != nil {
				m.logger.Errorw("failed to turn off motor", "error", err)
			}
		}
	})

	return nil
}

// cancelWaitProcesses provides the interrupt protocol for the time based GoFor implmenetation.
func (m *Motor) cancelWaitProcesses() {
	if m.cancelForFunc != nil {
		m.cancelForFunc()
		<-m.waitCh
	}
}

// IsPowered returns if the motor is currently on or off.
func (m *Motor) IsPowered(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down, by setting the appropriate pins to low states.
func (m *Motor) Stop(ctx context.Context) error {
	m.on = false
	return m.setPWM(ctx, 0)
}

// GoTo is not supported.
func (m *Motor) GoTo(ctx context.Context, rpm float64, positionRevolutions float64) error {
	return errors.New("not supported")
}

// ResetZeroPosition is not supported.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	return errors.New("not supported")
}
