// Package gpio implements a GPIO based motor.
package gpio

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
)

// WrapMotorWithEncoder takes a motor and adds an encoder onto it in order to understand its odometry.
func WrapMotorWithEncoder(
	ctx context.Context,
	e encoder.Encoder,
	c resource.Config,
	mc Config,
	m motor.Motor,
	logger logging.Logger,
) (motor.Motor, error) {
	if e == nil {
		return m, nil
	}

	if mc.TicksPerRotation < 0 {
		return nil, resource.NewConfigValidationError("", errors.New("ticks_per_rotation should be positive or zero"))
	}

	if mc.TicksPerRotation == 0 {
		mc.TicksPerRotation = 1
	}

	mm, err := newEncodedMotor(c.ResourceName(), mc, m, e, logger)
	if err != nil {
		return nil, err
	}

	return mm, nil
}

// newEncodedMotor creates a new motor that supports an arbitrary source of encoder information.
func newEncodedMotor(
	name resource.Name,
	motorConfig Config,
	realMotor motor.Motor,
	realEncoder encoder.Encoder,
	logger logging.Logger,
) (*EncodedMotor, error) {
	localReal, err := resource.AsType[motor.Motor](realMotor)
	if err != nil {
		return nil, err
	}

	if motorConfig.TicksPerRotation == 0 {
		motorConfig.TicksPerRotation = 1
	}

	em := &EncodedMotor{
		Named:            name.AsNamed(),
		cfg:              motorConfig,
		ticksPerRotation: float64(motorConfig.TicksPerRotation),
		real:             localReal,
		rampRate:         motorConfig.RampRate,
		maxPowerPct:      motorConfig.MaxPowerPct,
		logger:           logger,
		opMgr:            operation.NewSingleOperationManager(),
	}

	em.encoder = realEncoder

	// TODO DOCS-1524: link to docs that explain control parameters
	em.logger.Warn(
		"recommended: for more accurate motor control, configure 'control_parameters' in the motor config")

	if em.rampRate < 0 {
		return nil, fmt.Errorf("ramp rate can not be a negative number but is %v", em.rampRate)
	}

	if em.rampRate == 0 || em.rampRate > 0.5 {
		em.rampRate = 0.05 // Use a conservative value by default.
		em.logger.Warnf("setting ramp rate to default value of 0.05 instead of %v", em.rampRate)
	}

	if em.maxPowerPct < 0 || em.maxPowerPct > 1 {
		return nil, fmt.Errorf("max power pct needs to be (0, 1] but is %v", em.maxPowerPct)
	}
	if em.maxPowerPct == 0 {
		em.maxPowerPct = 1.0
	}

	return em, nil
}

// EncodedMotor is a motor that utilizes an encoder to track its position.
type EncodedMotor struct {
	resource.Named
	resource.AlwaysRebuild

	activeBackgroundWorkers sync.WaitGroup
	cfg                     Config
	real                    motor.Motor
	encoder                 encoder.Encoder
	offsetInTicks           float64

	mu                  sync.RWMutex
	makeAdjustmentsDone func()

	// how fast as we increase power do we do so
	// valid numbers are (0, 1]
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate         float64
	maxPowerPct      float64
	ticksPerRotation float64

	logger logging.Logger
	opMgr  *operation.SingleOperationManager
}

// makeAdjustments keeps track of the desired RPM and position.
func (m *EncodedMotor) makeAdjustments(ctx context.Context, goalRPM, goalPos, direction float64) error {
	lastTicks, _, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, nil)
	if err != nil {
		return err
	}
	lastTime := time.Now().UnixNano()
	_, lastPowerPct, err := m.real.IsPowered(ctx, nil)
	if err != nil {
		m.logger.Error(err)
		return err
	}
	lastPowerPct = math.Abs(lastPowerPct) * direction
	zeroRPMTracker := 0
	for {
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			// do not return context canceled
			return nil
		case <-timer.C:
		}

		currentTicks, _, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, nil)
		if err != nil {
			m.logger.CInfo(ctx, "error getting encoder position, sleeping then continuing: %w", err)
			if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
				m.logger.CInfo(ctx, "error sleeping, giving up %w", ctx.Err())
				return err
			}
			continue
		}
		now := time.Now().UnixNano()
		if (goalPos-currentTicks)*direction < 0 {
			// stop motor when at or past goal position
			return m.Stop(ctx, nil)
		}

		// calculate RPM based on change in position and change in time
		deltaPos := (currentTicks - lastTicks) / m.ticksPerRotation
		// time is polled in nanoseconds, convert to minutes for rpm
		deltaTime := (float64(now) - float64(lastTime)) / float64(6e10)
		var currentRPM float64
		if deltaTime == 0.0 {
			currentRPM = 0
		} else {
			currentRPM = deltaPos / deltaTime
			if currentRPM == 0 {
				zeroRPMTracker++
			}
		}
		if zeroRPMTracker > 10 {
			m.logger.Errorf("error running motor %v, desired rpm is too low for stable motion: %v", m.Name().Name, goalRPM)
		}

		newPower, err := m.calcNewPowerPct(ctx, currentRPM, goalRPM, lastPowerPct, direction)
		if err != nil {
			return err
		}

		m.logger.CDebug(ctx, "making adjustments")
		m.logger.CDebugf(ctx, "currentRPM: %v, goalRPM: %v", currentRPM, goalRPM)
		m.logger.CDebugf(ctx, "lastTicks: %v, pos: %v, goalPos: %v", lastTicks, currentTicks, goalPos)
		m.logger.CDebugf(ctx, "lastTime: %v, now: %v", lastTime, now)

		lastTicks = currentTicks
		lastTime = now
		lastPowerPct = newPower
	}
}

// calcNewPowerPct does the math required to see if the RPM is too high or too low,
// and calculates the new power percent needed.
func (m *EncodedMotor) calcNewPowerPct(
	ctx context.Context, currentRPM, goalRPM, lastPowerPct, direction float64,
) (float64, error) {
	newPowerPct := lastPowerPct

	rpmErr := goalRPM - currentRPM
	// adjust our power based on the error in rpm
	// this does not depend on the motor position
	newPowerPct += (m.rampRate * sign(rpmErr))

	// prevents the motor from reversing
	if sign(newPowerPct) != direction {
		newPowerPct = lastPowerPct
	}

	newPowerPct = fixPowerPct(newPowerPct, m.maxPowerPct)

	if err := m.real.SetPower(ctx, newPowerPct, nil); err != nil {
		return 0, err
	}
	return newPowerPct, nil
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational.
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	if m.makeAdjustmentsDone != nil {
		m.makeAdjustmentsDone()
	}
	powerPct = fixPowerPct(powerPct, m.maxPowerPct)
	return m.real.SetPower(ctx, powerPct, nil)
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are
// negative the motor will spin in the forward direction.
// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
func (m *EncodedMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()
	currentTicks, posType, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return err
	}
	if err := checkEncPosType(posType); err != nil {
		return err
	}

	warning, err := motor.CheckSpeed(rpm, m.cfg.MaxRPM)
	if warning != "" {
		m.logger.CWarn(ctx, warning)
	}
	if err != nil {
		return err
	}

	if err := motor.CheckRevolutions(revolutions); err != nil {
		return err
	}

	goalPos, goalRPM, direction := encodedGoForMath(rpm, revolutions, currentTicks, m.ticksPerRotation)

	if err := m.goForInternal(goalRPM, goalPos, direction); err != nil {
		return err
	}

	positionReached := func(ctx context.Context) (bool, error) {
		var errs error
		currentTicks, _, posErr := m.encoder.Position(ctx, encoder.PositionTypeTicks, extra)
		errs = multierr.Combine(errs, posErr)
		if (goalPos-currentTicks)*direction < 0 {
			stopErr := m.Stop(ctx, extra)
			errs = multierr.Combine(errs, stopErr)
			return true, errs
		}
		return false, errs
	}
	err = m.opMgr.WaitForSuccess(
		ctx,
		10*time.Millisecond,
		positionReached,
	)
	// Ignore the context canceled error - this occurs when the adjustmentsCtx is canceled
	// with m.makeAdjustmentsDone in goForInternal and in Stop
	if !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (m *EncodedMotor) goForInternal(rpm, goalPos, direction float64) error {
	// cancel makeAdjustments if it already exists
	if m.makeAdjustmentsDone != nil {
		m.makeAdjustmentsDone()
	}

	// start a new makeAdjustments
	var adjustmentsCtx context.Context
	adjustmentsCtx, m.makeAdjustmentsDone = context.WithCancel(context.Background())
	m.activeBackgroundWorkers.Add(1)
	go func() {
		defer m.activeBackgroundWorkers.Done()
		_, lastPowerPct, err := m.real.IsPowered(adjustmentsCtx, nil)
		if err != nil {
			m.logger.Error(err)
			return
		}

		if err := m.real.SetPower(adjustmentsCtx, calcStartingPower(lastPowerPct, direction), nil); err != nil {
			m.logger.Error(err)
			return
		}

		if err := m.makeAdjustments(adjustmentsCtx, rpm, goalPos, direction); err != nil {
			m.logger.Error(err)
		}
	}()

	return nil
}

func calcStartingPower(currPower, direction float64) float64 {
	defaultPower := .2
	if sign(currPower) != direction {
		return defaultPower * direction
	}
	if math.Abs(currPower) < defaultPower {
		return defaultPower * direction
	}
	return currPower
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position
// This will block until the position has been reached.
func (m *EncodedMotor) GoTo(ctx context.Context, rpm, targetPosition float64, extra map[string]interface{}) error {
	currRotations, err := m.Position(ctx, extra)
	if err != nil {
		return err
	}
	rotations := targetPosition - currRotations

	// ignore the direction of rpm
	rpm = math.Abs(rpm)

	// if you call GoFor with 0 revolutions, the motor will spin forever. If we are at the target,
	// we must avoid this by not calling GoFor.
	if rdkutils.Float64AlmostEqual(rotations, 0, 0.1) {
		m.logger.CDebug(ctx, "GoTo distance nearly zero, not moving")
		return nil
	}
	return m.GoFor(ctx, rpm, rotations, extra)
}

// SetRPM instructs the motor to move at the specified RPM indefinitely.
func (m *EncodedMotor) SetRPM(ctx context.Context, rpm float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	warning, err := motor.CheckSpeed(rpm, m.cfg.MaxRPM)
	if warning != "" {
		m.logger.CWarn(ctx, warning)
	}
	if err != nil {
		return err
	}

	goalPos := math.Inf(int(rpm))
	direction := sign(rpm)
	if err := m.goForInternal(rpm, goalPos, direction); err != nil {
		return err
	}

	return nil
}

// ResetZeroPosition sets the current position (+/- offset) to be the new zero (home) position.
func (m *EncodedMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	if err := m.Stop(ctx, extra); err != nil {
		return err
	}
	if err := m.encoder.ResetPosition(ctx, extra); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.offsetInTicks = -1 * offset * m.ticksPerRotation
	return nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *EncodedMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, posType, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return 0, err
	}
	if err := checkEncPosType(posType); err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	pos := ticks + m.offsetInTicks
	return pos / m.ticksPerRotation, nil
}

// Properties returns whether or not the motor supports certain optional properties.
func (m *EncodedMotor) Properties(ctx context.Context, extra map[string]interface{}) (motor.Properties, error) {
	return motor.Properties{
		PositionReporting: true,
	}, nil
}

// IsPowered returns whether or not the motor is currently on, and the percent power (between 0
// and 1, if the motor is off then the percent power will be 0).
func (m *EncodedMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return m.real.IsPowered(ctx, extra)
}

// IsMoving returns if the motor is moving or not.
func (m *EncodedMotor) IsMoving(ctx context.Context) (bool, error) {
	return m.real.IsMoving(ctx)
}

// Stop stops makeAdjustments and stops the real motor.
func (m *EncodedMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	if m.makeAdjustmentsDone != nil {
		m.makeAdjustmentsDone()
	}
	return m.real.Stop(ctx, nil)
}

// Close cleanly shuts down the motor.
func (m *EncodedMotor) Close(ctx context.Context) error {
	if err := m.Stop(ctx, nil); err != nil {
		return err
	}
	m.activeBackgroundWorkers.Wait()
	return nil
}
