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
	"go.viam.com/rdk/utils"
)

// NewMotor constructs a new GPIO based motor on the given board using the
// given configuration.
func NewMotor(b board.Board, mc motor.Config, logger golog.Logger) (motor.Motor, error) {
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

	var pid motor.PID
	if mc.PID != nil {
		var err error
		pid, err = motor.CreatePID(mc.PID)
		if err != nil {
			return nil, err
		}
	}

	m = &Motor{
		Board:       b,
		A:           pins["a"],
		B:           pins["b"],
		Dir:         pins["dir"],
		PWM:         pins["pwm"],
		En:          pins["en"],
		on:          false,
		pwmFreq:     mc.PWMFreq,
		minPowerPct: mc.MinPowerPct,
		maxPowerPct: mc.MaxPowerPct,
		maxRPM:      mc.MaxRPM,
		dirFlip:     mc.DirFlip,
		pid:         pid,
		logger:      logger,
		cancelMu:    &sync.Mutex{},
	}
	return m, nil
}

var _ = motor.Motor(&Motor{})

// A Motor is a GPIO based Motor that resides on a GPIO Board.
type Motor struct {
	Board              board.Board
	A, B, Dir, PWM, En string
	on                 bool
	pwmFreq            uint
	minPowerPct        float64
	maxPowerPct        float64
	maxRPM             float64
	dirFlip            bool
	pid                motor.PID

	cancelMu      *sync.Mutex
	cancelForFunc func()
	waitCh        chan struct{}
	logger        golog.Logger
}

// PID return the underlying PID.
func (m *Motor) PID() motor.PID {
	return m.pid
}

// Position always returns 0.
func (m *Motor) Position(ctx context.Context) (float64, error) {
	return 0, nil
}

// PositionSupported always returns false.
func (m *Motor) PositionSupported(ctx context.Context) (bool, error) {
	return false, nil
}

// SetPower sets the associated pins (as discovered) and sets PWM to the given power percentage.
func (m *Motor) SetPower(ctx context.Context, powerPct float64) error {
	var errs error
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)

	if math.Abs(powerPct) <= 0.001 {
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
		m.Board.PWMSetFreq(ctx, pwmPin, m.pwmFreq),
		m.Board.PWMSet(ctx, pwmPin, byte(utils.ScaleByPct(255, powerPct))),
	)
}

// Go instructs the motor to operate at a certain power percentage from -1 to 1
// where the sign of the power dictates direction.
func (m *Motor) Go(ctx context.Context, powerPct float64) error {
	m.cancelMu.Lock()
	m.cancelWaitProcesses()
	m.cancelMu.Unlock()

	if math.Abs(powerPct) <= 0.001 {
		return m.Stop(ctx)
	}

	if m.Dir != "" {
		x := !math.Signbit(powerPct)
		if m.dirFlip {
			x = !x
		}
		return multierr.Combine(
			m.Board.GPIOSet(ctx, m.Dir, x),
			m.SetPower(ctx, powerPct),
		)
	}
	if m.A != "" && m.B != "" {
		return multierr.Combine(
			m.Board.GPIOSet(ctx, m.A, !math.Signbit(powerPct)),
			m.Board.GPIOSet(ctx, m.B, math.Signbit(powerPct)),
			m.SetPower(ctx, powerPct), // Must be last for A/B only drivers
		)
	}

	if !math.Signbit(powerPct) {
		return m.SetPower(ctx, powerPct)
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

// GoFor moves an inputted number of revolutations at the given rpm, no encoder is present
// for this so power is deteremiend via a linear relationship with the maxRPM and the distance
// traveled is a time based estimation based on desired RPM.
func (m *Motor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	if m.maxRPM == 0 {
		return errors.New("not supported, define maxRPM attribute")
	}

	powerPct, waitDur := goForMath(m.maxRPM, rpm, revolutions)
	err := m.Go(ctx, powerPct)
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

// IsOn returns if the motor is currently on or off.
func (m *Motor) IsOn(ctx context.Context) (bool, error) {
	return m.on, nil
}

// Stop turns the power to the motor off immediately, without any gradual step down, by setting the appropriate pins to low states.
func (m *Motor) Stop(ctx context.Context) error {
	m.on = false
	return m.SetPower(ctx, 0)
}

// GoTo is not supported.
func (m *Motor) GoTo(ctx context.Context, rpm float64, position float64) error {
	return errors.New("not supported")
}

// GoTillStop is not supported.
func (m *Motor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	return errors.New("not supported")
}

// ResetZeroPosition is not supported.
func (m *Motor) ResetZeroPosition(ctx context.Context, offset float64) error {
	return errors.New("not supported")
}
