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

var rpmDebug = false

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
		startedRPMMonitor: false,
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
		em.rampRate = 0.2 // Use a conservative value by default.
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

	stateMu sync.RWMutex
	state   EncodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu sync.Mutex

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

// EncodedMotorState is the core, non-statistical state for the motor.
// Multiple values should be updated atomically at the same time.
type EncodedMotorState struct {
	regulated    bool
	goalRPM      float64 // <= 0 means worker should do nothing
	lastPowerPct float64
	goalPos      float64
	direction    float64
}

// rpmMonitorStart starts the RPM monitor.
func (m *EncodedMotor) rpmMonitorStart(goalRPM, goalPos float64) {
	m.startedRPMMonitorMu.Lock()
	startedRPMMonitor := m.startedRPMMonitor
	m.startedRPMMonitorMu.Unlock()
	if startedRPMMonitor {
		return
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		m.rpmMonitor(goalRPM, goalPos)
	}, m.activeBackgroundWorkers.Done)
}

// rpmMonitor keeps track of the desired RPM and position.
func (m *EncodedMotor) rpmMonitor(goalRPM, goalPos float64) {
	if m.encoder == nil {
		panic("started rpmMonitor but have no encoder")
	}

	m.startedRPMMonitorMu.Lock()
	if m.startedRPMMonitor {
		m.startedRPMMonitorMu.Unlock()
		return
	}
	m.startedRPMMonitor = true
	m.startedRPMMonitorMu.Unlock()

	lastPos, err := m.position(m.cancelCtx, nil)
	if err != nil {
		panic(err)
	}
	lastTime := time.Now().UnixNano()

	for {
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-m.cancelCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		m.stateMu.Lock()
		if !m.state.regulated {
			m.stateMu.Unlock()
			continue
		}
		m.stateMu.Unlock()
		pos, err := m.position(m.cancelCtx, nil)
		if err != nil {
			m.logger.Info("error getting encoder position, sleeping then continuing: %w", err)
			if !utils.SelectContextOrWait(m.cancelCtx, 100*time.Millisecond) {
				m.logger.Info("error sleeping, giving up %w", m.cancelCtx.Err())
				return
			}
			continue
		}
		now := time.Now().UnixNano()

		lastPowerPct := m.state.lastPowerPct

		if (m.DirectionMoving() == 1 && pos >= goalPos) || (m.DirectionMoving() == -1 && pos <= goalPos) {
			// stop motor when at or past goal position
			if err := m.Stop(m.cancelCtx, nil); err != nil {
				m.logger.Error(err)
				return
			}
			continue
		}

		newPower, err := m.makeAdjustments(pos, lastPos, goalRPM, goalPos, lastPowerPct, now, lastTime)
		if err != nil {
			m.logger.Error(err)
			return
		}

		lastPos = pos
		lastTime = now

		m.state.lastPowerPct = newPower
	}
}

// makeAdjustments does the math required to see if the RPM is too high or too low,
// and if the goal position has been reached.
func (m *EncodedMotor) makeAdjustments(
	pos, lastPos, goalRPM, goalPos, lastPowerPct float64,
	now, lastTime int64,
) (newPowerPct float64, err error) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

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

	if rpmDebug {
		m.logger.Info("making adjustments")
		m.logger.Infof("lastPos: %v, pos: %v, goalPos: %v", lastPos, pos, goalPos)
		m.logger.Infof("lastTime: %v, now: %v", lastTime, now)
		m.logger.Infof("currentRPM: %v, goalRPM: %v", currentRPM, goalRPM)
	}

	dir := m.directionMovingInLock()

	if (dir == 1 && currentRPM > goalRPM) || (dir == -1 && currentRPM < goalRPM) {
		powerPct := lastPowerPct - (m.rampRate * dir)
		if sign(powerPct) != dir {
			powerPct = lastPowerPct
		}
		if rpmDebug {
			m.logger.Infof("decreasing powerPct to %v", powerPct)
		}
		if err := m.setPower(m.cancelCtx, powerPct, true); err != nil {
			return 0, err
		}
	}
	if (dir == 1 && currentRPM <= goalRPM) || (dir == -1 && currentRPM >= goalRPM) {
		newPowerPct := lastPowerPct + (m.rampRate * dir)
		if sign(newPowerPct) != dir {
			newPowerPct = lastPowerPct
		}
		if rpmDebug {
			m.logger.Infof("increasing powerPct to %v", newPowerPct)
		}
		if err := m.setPower(m.cancelCtx, newPowerPct, true); err != nil {
			return 0, err
		}
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
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return int64(m.directionMovingInLock())
}

func (m *EncodedMotor) directionMovingInLock() float64 {
	move, err := m.real.IsMoving(context.Background())
	if move {
		_, isSingle := m.encoder.(*single.Encoder)
		if sign(m.state.lastPowerPct) != sign(m.state.direction) && isSingle {
			// short sleep when changing directions to minimize lost ticks in single encoder
			time.Sleep(10 * time.Microsecond)
		}
		return sign(m.state.lastPowerPct)
	}
	if err != nil {
		m.logger.Error(err)
	}
	return 0
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational.
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.setPower(ctx, powerPct, false)
}

// setPower assumes the state lock is held.
func (m *EncodedMotor) setPower(ctx context.Context, powerPct float64, internal bool) error {
	dir := sign(powerPct)
	// If the control config exists, a control loop must exist, so the motor should be allowed to run at a power lower than 10%.
	// In the case that the motor is tuning, m.loop will be nil, but m.controlLoopConfig.Blocks will not be empty,
	// which is why m.loop is not checked here.
	if math.Abs(powerPct) < 0.1 && len(m.controlLoopConfig.Blocks) == 0 {
		m.state.lastPowerPct = 0.1 * dir
	} else {
		m.state.lastPowerPct = powerPct
	}
	if !internal {
		m.state.goalRPM = 0       // if we're setting power externally, don't control RPM
		m.state.regulated = false // user wants direct control, so we stop trying to control the world
	}
	m.state.lastPowerPct = fixPowerPct(m.state.lastPowerPct, m.maxPowerPct)
	return m.real.SetPower(ctx, m.state.lastPowerPct, nil)
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
	if err := m.goForInternal(ctx, rpm, revolutions); err != nil {
		return err
	}

	if revolutions == 0 {
		return nil
	}

	if m.loop != nil {
		m.stateMu.Lock()
		goal := m.state.goalPos
		m.stateMu.Unlock()

		positionReached := func(ctx context.Context) (bool, error) {
			var errs error
			pos, posErr := m.position(ctx, extra)
			errs = multierr.Combine(errs, posErr)
			if rdkutils.Float64AlmostEqual(pos, goal, 5.0) {
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

	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

func (m *EncodedMotor) goForInternal(ctx context.Context, rpm, revolutions float64) error {
	currentPos, err := m.position(ctx, nil)
	if err != nil {
		return err
	}
	direction := sign(rpm * revolutions)
	goalPos := (math.Abs(revolutions) * m.ticksPerRotation * direction) + currentPos
	goalRPM := math.Abs(rpm) * m.state.direction

	if m.loop == nil {
		// create new control loop if control config exists
		if len(m.controlLoopConfig.Blocks) != 0 {
			if err := m.startControlLoop(); err != nil {
				return err
			}
		} else {
			m.rpmMonitorStart(goalRPM, goalPos)
		}
	}

	m.state.direction = sign(rpm * revolutions)

	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		m.logger.CWarn(ctx, "motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	case m.cfg.MaxRPM > 0 && speed > m.cfg.MaxRPM-0.1:
		m.logger.CWarnf(ctx, "motor speed is nearly the max rev_per_min (%f)", m.cfg.MaxRPM)
	default:
	}

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	m.state.regulated = true
	if revolutions == 0 {
		if m.loop != nil {
			velVal := math.Abs(rpm * m.ticksPerRotation / 60)
			// when rev = 0, only velocity is controlled
			// setPoint is +/- infinity, maxVel is calculated velVal
			if err := m.updateControlBlock(ctx, math.Inf(int(rpm)), velVal); err != nil {
				return err
			}
		} else {
			// Moving 0 revolutions is a special value meaning "move forever."
			oldRpm := m.state.goalRPM
			m.state.goalRPM = rpm
			m.state.goalPos = math.Inf(int(rpm))
			// if we are already moving, let rpmMonitor deal with setPower
			if math.Abs(oldRpm) > 0.001 && direction == m.directionMovingInLock() {
				return nil
			}
			// if moving from stop, start at 10% power
			if err := m.setPower(ctx, direction*0.1, true); err != nil {
				return err
			}
		}
		return nil
	}

	if m.loop != nil {
		velVal := math.Abs(rpm * m.ticksPerRotation / 60)
		// when rev is not 0, velocity and position are controlled
		// setPoint is goalPos, maxVel is calculated velVal
		if err := m.updateControlBlock(ctx, goalPos, velVal); err != nil {
			return err
		}
	} else {
		startingPwr := 0.1 * direction
		err = m.setPower(ctx, startingPwr, true)
		if err != nil {
			return err
		}
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

	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.offsetInTicks = -1 * offset * m.ticksPerRotation
	return nil
}

// report position in ticks.
func (m *EncodedMotor) position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, _, err := m.encoder.Position(ctx, encoder.PositionTypeTicks, extra)
	if err != nil {
		return 0, err
	}
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
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
	m.stateMu.Lock()
	m.state.goalRPM = 0
	m.state.regulated = false
	m.stateMu.Unlock()

	// after the motor is created, Stop is called, but if the PID controller
	// is auto-tuning, the loop needs to keep running
	if m.loop != nil && !m.loop.GetTuning(ctx) {
		m.loop.Stop()
		m.loop = nil
	}
	return m.real.Stop(ctx, nil)
}

// Close cleanly shuts down the motor.
func (m *EncodedMotor) Close(ctx context.Context) error {
	if err := m.Stop(ctx, nil); err != nil {
		return err
	}
	m.cancel()
	m.activeBackgroundWorkers.Wait()
	return nil
}
