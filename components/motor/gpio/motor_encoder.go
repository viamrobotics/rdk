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
	"go.viam.com/rdk/components/encoder/single"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/control"
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

	single, isSingle := e.(*single.Encoder)
	if isSingle {
		single.AttachDirectionalAwareness(mm)
		logger.CInfo(ctx, "direction attached to single encoder from encoded motor")
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

	cancelCtx, cancel := context.WithCancel(context.Background())
	em := &EncodedMotor{
		Named:             name.AsNamed(),
		cfg:               motorConfig,
		ticksPerRotation:  float64(motorConfig.TicksPerRotation),
		real:              localReal,
		cancelCtx:         cancelCtx,
		cancel:            cancel,
		rampRate:          motorConfig.RampRate,
		maxPowerPct:       motorConfig.MaxPowerPct,
		logger:            logger,
		opMgr:             operation.NewSingleOperationManager(),
		loop:              nil,
		controlLoopConfig: control.Config{},
	}

	props, err := realEncoder.Properties(context.Background(), nil)
	if err != nil {
		return nil, errors.New("cannot get encoder properties")
	}
	if !props.TicksCountSupported {
		return nil,
			encoder.NewEncodedMotorPositionTypeUnsupportedError(props)
	}
	em.encoder = realEncoder

	// setup control loop
	if motorConfig.ControlParameters != nil {
		if err := em.setupControlLoop(); err != nil {
			return nil, err
		}
	} else {
		// TODO DOCS-1524: link to docs that explain control parameters
		em.logger.Warn(
			"recommended: for more accurate motor control, configure 'control_parameters' in the motor config")
	}

	if em.rampRate < 0 || em.rampRate > 1 {
		return nil, fmt.Errorf("ramp rate needs to be (0, 1] but is %v", em.rampRate)
	}
	if em.rampRate == 0 {
		em.rampRate = 0.05 // Use a conservative value by default.
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
	lastPowerPct            float64

	mu             sync.RWMutex
	rpmMonitorDone func()

	// how fast as we increase power do we do so
	// valid numbers are (0, 1]
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate         float64
	maxPowerPct      float64
	ticksPerRotation float64

	logger    logging.Logger
	cancelCtx context.Context
	cancel    func()
	opMgr     *operation.SingleOperationManager

	controlLoopConfig control.Config
	blockNames        map[string][]string
	loop              *control.Loop
}

// rpmMonitor keeps track of the desired RPM and position.
func (m *EncodedMotor) rpmMonitor(ctx context.Context, goalRPM, goalPos, direction float64) {
	lastPos, err := m.position(ctx, nil)
	if err != nil {
		panic(err)
	}
	lastTime := time.Now().UnixNano()
	lastPowerPct := 0.0

	for {
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		pos, err := m.position(ctx, nil)
		if err != nil {
			m.logger.CInfo(ctx, "error getting encoder position, sleeping then continuing: %w", err)
			if !utils.SelectContextOrWait(ctx, 100*time.Millisecond) {
				m.logger.CInfo(ctx, "error sleeping, giving up %w", ctx.Err())
				return
			}
			continue
		}
		now := time.Now().UnixNano()

		if (direction == 1 && pos >= goalPos) || (direction == -1 && pos <= goalPos) {
			// stop motor when at or past goal position
			if err := m.Stop(ctx, nil); err != nil {
				m.logger.CError(ctx, err)
				return
			}
			return
		}

		newPower, err := m.makeAdjustments(ctx, pos, lastPos, goalRPM, goalPos, lastPowerPct, direction, now, lastTime)
		if err != nil {
			m.logger.CError(ctx, err)
			return
		}

		lastPos = pos
		lastTime = now
		lastPowerPct = newPower
	}
}

// makeAdjustments does the math required to see if the RPM is too high or too low,
// and if the goal position has been reached.
func (m *EncodedMotor) makeAdjustments(
	ctx context.Context, pos, lastPos, goalRPM, goalPos, lastPowerPct, direction float64,
	now, lastTime int64,
) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	newPowerPct := lastPowerPct

	// calculate RPM based on change in position and change in time
	deltaPos := (pos - lastPos) / m.ticksPerRotation
	// time is polled in nanoseconds, convert to minutes for rpm
	deltaTime := (float64(now) - float64(lastTime)) / float64(6e10)
	var currentRPM float64
	if deltaTime == 0.0 {
		currentRPM = 0
	} else {
		currentRPM = deltaPos / deltaTime
	}

	m.logger.CDebug(ctx, "making adjustments")
	m.logger.CDebugf(ctx, "lastPos: %v, pos: %v, goalPos: %v", lastPos, pos, goalPos)
	m.logger.CDebugf(ctx, "lastTime: %v, now: %v", lastTime, now)
	m.logger.CDebugf(ctx, "currentRPM: %v, goalRPM: %v", currentRPM, goalRPM)

	rpmErr := goalRPM - currentRPM
	// adjust our power based on the error in rpm
	newPowerPct += (m.rampRate * sign(rpmErr))

	if sign(newPowerPct) != direction {
		newPowerPct = lastPowerPct
	}

	if err := m.setPower(ctx, newPowerPct); err != nil {
		return 0, err
	}
	return newPowerPct, nil
}

func fixPowerPct(powerPct, max float64) float64 {
	powerPct = math.Min(powerPct, max)
	powerPct = math.Max(powerPct, -1*max)
	return powerPct
}

func sign(x float64) float64 { // A quick helper function
	if math.Signbit(x) {
		return -1.0
	}
	return 1.0
}

// DirectionMoving returns the direction we are currently moving in, with 1 representing
// forward and  -1 representing backwards.
func (m *EncodedMotor) DirectionMoving() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(m.directionMovingInLock())
}

func (m *EncodedMotor) directionMovingInLock() float64 {
	move, err := m.real.IsMoving(context.Background())
	if move {
		return sign(m.lastPowerPct)
	}
	if err != nil {
		m.logger.Error(err)
	}
	return 0
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational.
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	if m.rpmMonitorDone != nil {
		m.rpmMonitorDone()
	}
	if m.loop != nil {
		m.loop.Pause()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.setPower(ctx, powerPct)
}

// setPower assumes the state lock is held.
func (m *EncodedMotor) setPower(ctx context.Context, powerPct float64) error {
	m.lastPowerPct = fixPowerPct(powerPct, m.maxPowerPct)
	return m.real.SetPower(ctx, m.lastPowerPct, nil)
}

// goForMath calculates goalPos, goalRPM, and direction based on the given GoFor rpm and revolutions, and the current position.
func (m *EncodedMotor) goForMath(ctx context.Context, rpm, revolutions float64) (float64, float64, float64) {
	direction := sign(rpm * revolutions)

	currentPos, err := m.position(ctx, nil)
	if err != nil {
		m.logger.CError(ctx, err)
	}
	goalPos := (math.Abs(revolutions) * m.ticksPerRotation * direction) + currentPos
	goalRPM := math.Abs(rpm) * direction

	if revolutions == 0 {
		goalPos = math.Inf(int(direction))
	}

	return goalPos, goalRPM, direction
}

// GoFor instructs the motor to go in a specific direction for a specific amount of
// revolutions at a given speed in revolutions per minute. Both the RPM and the revolutions
// can be assigned negative values to move in a backwards direction. Note: if both are
// negative the motor will spin in the forward direction.
// If revolutions is 0, this will run the motor at rpm indefinitely
// If revolutions != 0, this will block until the number of revolutions has been completed or another operation comes in.
func (m *EncodedMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	goalPos, goalRPM, direction := m.goForMath(ctx, rpm, revolutions)

	if err := m.goForInternal(ctx, goalRPM, goalPos, direction); err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}

	positionReached := func(ctx context.Context) (bool, error) {
		var errs error
		pos, posErr := m.position(ctx, extra)
		errs = multierr.Combine(errs, posErr)
		if rdkutils.Float64AlmostEqual(pos, goalPos, 10.0) {
			stopErr := m.Stop(ctx, extra)
			errs = multierr.Combine(errs, stopErr)
			return true, errs
		}
		return false, errs
	}
	err := m.opMgr.WaitForSuccess(
		ctx,
		10*time.Millisecond,
		positionReached,
	)
	// Ignore the context canceled error - this occurs when the motor is stopped
	// at the beginning of goForInternal
	if !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}

func (m *EncodedMotor) goForInternal(ctx context.Context, rpm, goalPos, direction float64) error {
	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		m.logger.CWarn(ctx, "motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	case m.cfg.MaxRPM > 0 && speed > m.cfg.MaxRPM-0.1:
		m.logger.CWarnf(ctx, "motor speed is nearly the max rev_per_min (%f)", m.cfg.MaxRPM)
	default:
	}

	if m.loop == nil {
		// create new control loop if control config exists
		if len(m.controlLoopConfig.Blocks) != 0 {
			if err := m.startControlLoop(); err != nil {
				return err
			}
		} else {
			// cancel rpmMonitor if it already exists
			if m.rpmMonitorDone != nil {
				m.rpmMonitorDone()
			}

			// start a new rpmMonitor
			var rpmCtx context.Context
			rpmCtx, m.rpmMonitorDone = context.WithCancel(context.Background())
			m.activeBackgroundWorkers.Add(1)
			utils.ManagedGo(func() {
				m.rpmMonitor(rpmCtx, rpm, goalPos, direction)
			}, m.activeBackgroundWorkers.Done)

			return nil
		}
	}

	m.loop.Resume()
	// set control loop values
	velVal := math.Abs(rpm * m.ticksPerRotation / 60)
	// when rev = 0, only velocity is controlled
	// setPoint is +/- infinity, maxVel is calculated velVal
	if err := m.updateControlBlock(ctx, goalPos, velVal); err != nil {
		return err
	}

	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position
// This will block until the position has been reached.
func (m *EncodedMotor) GoTo(ctx context.Context, rpm, targetPosition float64, extra map[string]interface{}) error {
	pos, err := m.position(ctx, extra)
	if err != nil {
		return err
	}
	currRotations := pos / m.ticksPerRotation
	rotations := targetPosition - currRotations
	// if you call GoFor with 0 revolutions, the motor will spin forever. If we are at the target,
	// we must avoid this by not calling GoFor.
	if rdkutils.Float64AlmostEqual(rotations, 0, 0.1) {
		m.logger.CDebug(ctx, "GoTo distance nearly zero, not moving")
		return nil
	}
	return m.GoFor(ctx, rpm, rotations, extra)
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

// report position in ticks.
func (m *EncodedMotor) position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, _, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	pos := ticks + m.offsetInTicks
	return pos, nil
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *EncodedMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, err := m.position(ctx, extra)
	if err != nil {
		return 0, err
	}

	return ticks / m.ticksPerRotation, nil
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

// Stop stops rpmMonitor and stops the real motor.
func (m *EncodedMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	// after the motor is created, Stop is called, but if the PID controller
	// is auto-tuning, the loop needs to keep running
	if m.loop != nil && !m.loop.GetTuning(ctx) {
		m.loop.Pause()
	}
	if m.rpmMonitorDone != nil {
		m.rpmMonitorDone()
	}
	return m.real.Stop(ctx, nil)
}

// Close cleanly shuts down the motor.
func (m *EncodedMotor) Close(ctx context.Context) error {
	if err := m.Stop(ctx, nil); err != nil {
		return err
	}
	if m.loop != nil {
		m.loop.Stop()
		m.loop = nil
	}
	m.cancel()
	m.activeBackgroundWorkers.Wait()
	return nil
}
