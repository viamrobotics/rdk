// Package gpio implements a GPIO based motor.
package gpio

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/encoder/single"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/operation"
	"go.viam.com/rdk/resource"
	rdkutils "go.viam.com/rdk/utils"
	"go.viam.com/utils"
)

// WrapMotorWithEncoder takes a motor and adds an encoder onto it in order to understand its odometry.
func WrapMotorWithEncoder(
	ctx context.Context,
	e encoder.Encoder,
	c resource.Config,
	mc Config,
	m motor.Motor,
	logger golog.Logger,
) (motor.Motor, error) {
	if e == nil {
		return m, nil
	}

	if mc.TicksPerRotation < 0 {
		return nil, utils.NewConfigValidationError("", errors.New("ticks_per_rotation should be positive or zero"))
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
		logger.Info("direction attached to single encoder from encoded motor")
	}

	mm.RPMMonitorStart()

	return mm, nil
}

// NewEncodedMotor creates a new motor that supports an arbitrary source of encoder information.
func NewEncodedMotor(
	conf resource.Config,
	motorConfig Config,
	realMotor motor.Motor,
	encoder encoder.Encoder,
	logger golog.Logger,
) (motor.Motor, error) {
	return newEncodedMotor(conf.ResourceName(), motorConfig, realMotor, encoder, logger)
}

func newEncodedMotor(
	name resource.Name,
	motorConfig Config,
	realMotor motor.Motor,
	realEncoder encoder.Encoder,
	logger golog.Logger,
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
		ticksPerRotation:  int64(motorConfig.TicksPerRotation),
		real:              localReal,
		cancelCtx:         cancelCtx,
		cancel:            cancel,
		rampRate:          motorConfig.RampRate,
		maxPowerPct:       motorConfig.MaxPowerPct,
		logger:            logger,
		opMgr:             operation.NewSingleOperationManager(),
		startedRPMMonitor: false,
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

	if len(motorConfig.ControlLoop.Blocks) != 0 {
		cLoop, err := control.NewLoop(logger, motorConfig.ControlLoop, em)
		if err != nil {
			return nil, err
		}
		err = cLoop.Start()
		if err != nil {
			return nil, err
		}
		em.loop = cLoop
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

	em.flip = 1
	if motorConfig.DirectionFlip {
		em.flip = -1
	}

	// _rpmDebug = motorConfig.Debug

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

	stateMu sync.RWMutex
	state   EncodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu sync.Mutex

	// how fast as we increase power do we do so
	// valid numbers are (0, 1]
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate         float64
	maxPowerPct      float64
	flip             int64 // defaults to 1, becomes -1 if the motor config has a true DirectionFLip bool
	ticksPerRotation int64

	rpmMonitorCalls int64
	logger          golog.Logger
	cancelCtx       context.Context
	cancel          func()
	loop            *control.Loop
	opMgr           *operation.SingleOperationManager
}

// EncodedMotorState is the core, non-statistical state for the motor.
// Multiple values should be updated atomically at the same time.
type EncodedMotorState struct {
	regulated    bool
	desiredRPM   float64 // <= 0 means worker should do nothing
	currentRPM   float64
	lastPowerPct float64
	setPoint     int64
	goalPos      float64
}

// RPMMonitorStart starts the RPM monitor.
func (m *EncodedMotor) RPMMonitorStart() {
	m.startedRPMMonitorMu.Lock()
	startedRPMMonitor := m.startedRPMMonitor
	m.startedRPMMonitorMu.Unlock()
	if startedRPMMonitor {
		return
	}
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		m.rpmMonitor()
	}, m.activeBackgroundWorkers.Done)
}

// rpmMonitor keeps track of the desired RPM and position
func (m *EncodedMotor) rpmMonitor() {
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

	lastPos, _, err := m.encoder.Position(m.cancelCtx, encoder.PositionTypeUnspecified, nil)
	if err != nil {
		panic(err)
	}
	lastTime := time.Now().UnixNano()

	for {
		m.stateMu.Lock()
		if !m.state.regulated {
			m.stateMu.Unlock()
			continue
		}
		m.stateMu.Unlock()
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-m.cancelCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		pos, _, err := m.encoder.Position(m.cancelCtx, encoder.PositionTypeUnspecified, nil)
		if err != nil {
			m.logger.Info("error getting encoder position, sleeping then continuing: %w", err)
			if !utils.SelectContextOrWait(m.cancelCtx, 100*time.Millisecond) {
				m.logger.Info("error sleeping, giving up %w", m.cancelCtx.Err())
				return
			}
			continue
		}
		if (m.DirectionMoving() == 1 && pos >= m.state.goalPos) || (m.DirectionMoving() == -1 && pos <= m.state.goalPos) {
			// stop motor when at or past goal position
			m.Stop(m.cancelCtx, nil)
			continue
		}
		now := time.Now().UnixNano()

		m.makeAdjustments(pos, lastPos, now, lastTime)

		lastPos = pos
		lastTime = now
	}
}

// makeAdjustments does the math required to see if the RPM is too high or too low,
// and if the goal position has been reached
func (m *EncodedMotor) makeAdjustments(pos, lastPos float64, now, lastTime int64) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	// calculate RPM based on change in position and change in time
	deltaPos := (pos - lastPos) / float64(m.ticksPerRotation)
	deltaTime := (float64(now) - float64(lastTime)) / float64(6e10)
	if deltaTime == 0 {
		m.state.currentRPM = 0
	}
	m.state.currentRPM = deltaPos / deltaTime

	if m.state.currentRPM > m.state.desiredRPM {
		m.state.lastPowerPct -= 0.1
		if err := m.setPower(m.cancelCtx, m.state.lastPowerPct, true); err != nil {
			panic(err)
		}
	}
	if m.state.currentRPM < m.state.desiredRPM {
		m.state.lastPowerPct += 0.1
		if err := m.setPower(m.cancelCtx, m.state.lastPowerPct, true); err != nil {
			panic(err)
		}
	}
	atomic.AddInt64(&m.rpmMonitorCalls, 1)

	return
}

// RPMMonitorCalls returns the number of calls RPM monitor has made.
func (m *EncodedMotor) RPMMonitorCalls() int64 {
	return atomic.LoadInt64(&m.rpmMonitorCalls)
}

// IsRegulated returns if the motor is currently regulated or not.
func (m *EncodedMotor) IsRegulated() bool {
	m.stateMu.RLock()
	regulated := m.state.regulated
	m.stateMu.RUnlock()
	return regulated
}

// SetRegulated sets if the motor should be regulated.
func (m *EncodedMotor) SetRegulated(b bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state.regulated = b
}

func (m *EncodedMotor) fixPowerPct(powerPct float64) float64 {
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)
	return powerPct
}

func sign(x float64) int64 { // A quick helper function
	if math.Signbit(x) {
		return -1
	}
	return 1
}

// DirectionMoving returns the direction we are currently moving in, with 1 representing
// forward and  -1 representing backwards.
func (m *EncodedMotor) DirectionMoving() int64 {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.directionMovingInLock()
}

func (m *EncodedMotor) directionMovingInLock() int64 {
	if move, _ := m.real.IsMoving(context.Background()); move {
		return sign(m.state.lastPowerPct)
	}
	return 0
}

// SetPower sets the percentage of power the motor should employ between -1 and 1.
// Negative power implies a backward directional rotational
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.setPower(ctx, powerPct, false)
}

// setPower assumes the state lock is held.
func (m *EncodedMotor) setPower(ctx context.Context, powerPct float64, internal bool) error {
	m.state.lastPowerPct = powerPct
	if !internal {
		m.state.desiredRPM = 0    // if we're setting power externally, don't control RPM
		m.state.regulated = false // user wants direct control, so we stop trying to control the world
	}
	m.state.lastPowerPct = m.fixPowerPct(powerPct)
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

	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

func (m *EncodedMotor) goForInternal(ctx context.Context, rpm, revolutions float64) error {
	// m.RPMMonitorStart()
	rpm *= float64(m.flip)
	dir := sign(rpm * revolutions)

	switch speed := math.Abs(rpm); {
	case speed < 0.1:
		m.logger.Warn("motor speed is nearly 0 rev_per_min")
		return motor.NewZeroRPMError()
	case m.cfg.MaxRPM > 0 && speed > m.cfg.MaxRPM-0.1:
		m.logger.Warnf("motor speed is nearly the max rev_per_min (%f)", m.cfg.MaxRPM)
	default:
	}

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if revolutions == 0 {
		// Moving 0 revolutions is a special value meaning "move forever."
		oldRpm := m.state.desiredRPM
		m.state.desiredRPM = rpm
		m.state.regulated = false // we're not going to a position

		// if we are already moving, let rpmMonitor deal with setPower
		if math.Abs(oldRpm) > 0.001 && dir == m.directionMovingInLock() {
			return nil
		}
		// if moving from stop, start at 10% power
		err := m.setPower(ctx, float64(dir)*0.1, true) // power of 6% is random
		return err
	}
	m.state.regulated = true

	currentPos, _, err := m.encoder.Position(ctx, encoder.PositionTypeUnspecified, nil)
	if err != nil {
		return err
	}
	goalPos := (revolutions * float64(m.ticksPerRotation)) + currentPos

	m.state.desiredRPM = rpm
	m.state.goalPos = goalPos

	startingPwr := 0.1 * float64(dir)
	err = m.setPower(ctx, startingPwr, true)
	if err != nil {
		return err
	}
	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target/position
// This will block until the position has been reached
func (m *EncodedMotor) GoTo(ctx context.Context, rpm, targetPosition float64, extra map[string]interface{}) error {
	rpm = math.Abs(rpm) * float64(m.flip)
	pos, _, err := m.encoder.Position(ctx, encoder.PositionTypeUnspecified, extra)
	if err != nil {
		return err
	}
	currRotations := pos / float64(m.ticksPerRotation)
	rotations := targetPosition - currRotations
	// if you call GoFor with 0 revolutions, the motor will spin forever. If we are at the target,
	// we must avoid this by not calling GoFor.
	if rdkutils.Float64AlmostEqual(rotations, 0, 0.1) {
		m.logger.Debug("GoTo distance nearly zero, not moving")
		return nil
	}
	return m.GoFor(ctx, rpm, rotations, extra)
}

// Set the current position (+/- offset) to be the new zero (home) position.
func (m *EncodedMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return m.encoder.ResetPosition(ctx, extra)
}

// Position reports the position of the motor based on its encoder. If it's not supported, the returned
// data is undefined. The unit returned is the number of revolutions which is intended to be fed
// back into calls of GoFor.
func (m *EncodedMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, _, err := m.encoder.Position(ctx, encoder.PositionTypeUnspecified, extra)
	if err != nil {
		return 0, err
	}

	return ticks / float64(m.ticksPerRotation), nil
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

func (m *EncodedMotor) off(ctx context.Context) error {
	m.state.desiredRPM = 0
	m.state.regulated = false
	return m.real.Stop(ctx, nil)
}

func (m *EncodedMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.off(ctx)
}

// Close cleanly shuts down the motor.
func (m *EncodedMotor) Close(ctx context.Context) error {
	m.cancel()
	m.activeBackgroundWorkers.Wait()
	return nil
}
