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
	"go.viam.com/utils"

	"go.viam.com/rdk/components/encoder"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/control"
	"go.viam.com/rdk/operation"
	rutils "go.viam.com/rdk/utils"
)

var (
	_rpmDebugMu sync.Mutex
	_rpmSleep   = 50 * time.Millisecond // really just for testing
	_rpmDebug   = false
)

func getRPMSleepDebug() (time.Duration, bool) {
	_rpmDebugMu.Lock()
	defer _rpmDebugMu.Unlock()
	return _rpmSleep, _rpmDebug
}

// SetRPMSleepDebug is for testing only.
func SetRPMSleepDebug(dur time.Duration, debug bool) func() {
	_rpmDebugMu.Lock()
	defer _rpmDebugMu.Unlock()
	prevRPMSleep := _rpmSleep
	prevRPMDebug := _rpmDebug
	_rpmSleep = dur
	_rpmDebug = debug
	return func() {
		SetRPMSleepDebug(prevRPMSleep, prevRPMDebug)
	}
}

// WrapMotorWithEncoder takes a motor and adds an encoder onto it in order to understand its odometry.
func WrapMotorWithEncoder(
	ctx context.Context,
	e encoder.Encoder,
	c config.Component,
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

	mm, err := newEncodedMotor(c, mc, m, e, logger)
	if err != nil {
		return nil, err
	}

	single, isSingle := rutils.UnwrapProxy(e).(*encoder.SingleEncoder)
	if isSingle {
		single.AttachDirectionalAwareness(mm)
		logger.Info("direction attached to single encoder from encoded motor")
	}

	mm.RPMMonitorStart()

	return mm, nil
}

// NewEncodedMotor creates a new motor that supports an arbitrary source of encoder information.
func NewEncodedMotor(
	config config.Component,
	motorConfig Config,
	realMotor motor.Motor,
	encoder encoder.Encoder,
	logger golog.Logger,
) (motor.LocalMotor, error) {
	return newEncodedMotor(config, motorConfig, realMotor, encoder, logger)
}

func newEncodedMotor(
	config config.Component,
	motorConfig Config,
	realMotor motor.Motor,
	realEncoder encoder.Encoder,
	logger golog.Logger,
) (*EncodedMotor, error) {
	localReal, ok := realMotor.(motor.LocalMotor)
	if !ok {
		return nil, motor.NewUnimplementedLocalInterfaceError(realMotor)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	em := &EncodedMotor{
		activeBackgroundWorkers: &sync.WaitGroup{},
		cfg:                     motorConfig,
		real:                    localReal,
		encoder:                 realEncoder,
		cancelCtx:               cancelCtx,
		cancel:                  cancel,
		stateMu:                 &sync.RWMutex{},
		startedRPMMonitorMu:     &sync.Mutex{},
		rampRate:                motorConfig.RampRate,
		maxPowerPct:             motorConfig.MaxPowerPct,
		logger:                  logger,
		loop:                    nil,
	}

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

	if val, ok := config.Attributes["rpmDebug"]; ok {
		if val == "true" {
			_rpmDebug = true
		}
	}

	return em, nil
}

// EncodedMotor is a motor that utilizes an encoder to track its position.
type EncodedMotor struct {
	activeBackgroundWorkers *sync.WaitGroup
	cfg                     Config
	real                    motor.LocalMotor
	encoder                 encoder.Encoder

	stateMu *sync.RWMutex
	state   EncodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu *sync.Mutex

	// how fast as we increase power do we do so
	// valid numbers are (0, 1]
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate    float64
	maxPowerPct float64
	flip        int64 // defaults to 1, becomes -1 if the motor config has a true DirectionFLip bool

	rpmMonitorCalls int64
	logger          golog.Logger
	cancelCtx       context.Context
	cancel          func()
	loop            *control.Loop
	opMgr           operation.SingleOperationManager

	generic.Unimplemented
}

// EncodedMotorState is the core, non-statistical state for the motor.
// Multiple values should be updated atomically at the same time.
type EncodedMotorState struct {
	regulated    bool
	desiredRPM   float64 // <= 0 means worker should do nothing
	currentRPM   float64
	lastPowerPct float64
	setPoint     int64
}

// Position returns the position of the motor.
func (m *EncodedMotor) Position(ctx context.Context, extra map[string]interface{}) (float64, error) {
	ticks, err := m.encoder.TicksCount(ctx, extra)
	if err != nil {
		return 0, err
	}

	return ticks / float64(m.cfg.TicksPerRotation), nil
}

// DirectionMoving returns the direction we are currently mpving in, with 1 representing
// forward and  -1 representing backwards.
func (m *EncodedMotor) DirectionMoving() int64 {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.directionMovingInLock()
}

func sign(x float64) int64 { // A quick helper function
	if math.Signbit(x) {
		return -1
	}
	return 1
}

func (m *EncodedMotor) directionMovingInLock() int64 {
	return sign(m.state.lastPowerPct)
}

// Properties returns the status of whether the motor supports certain optional features.
func (m *EncodedMotor) Properties(ctx context.Context, extra map[string]interface{}) (map[motor.Feature]bool, error) {
	return map[motor.Feature]bool{
		motor.PositionReporting: true,
	}, nil
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

// SetPower sets the power of the motor to the given percentage value between 0 and 1.
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64, extra map[string]interface{}) error {
	m.opMgr.CancelRunning(ctx)
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.setPower(ctx, powerPct, false)
}

// setPower assumes the state lock is held.
func (m *EncodedMotor) setPower(ctx context.Context, powerPct float64, internal bool) error {
	if !internal {
		m.state.desiredRPM = 0    // if we're setting power externally, don't control RPM
		m.state.regulated = false // user wants direct control, so we stop trying to control the world
	}
	m.state.lastPowerPct = m.fixPowerPct(powerPct)
	return m.real.SetPower(ctx, m.state.lastPowerPct, nil)
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

	lastPosFl, err := m.encoder.TicksCount(m.cancelCtx, nil)
	if err != nil {
		panic(err)
	}
	lastPos := int64(lastPosFl)
	lastTime := time.Now().UnixNano()

	rpmSleep, rpmDebug := getRPMSleepDebug()
	for {
		timer := time.NewTimer(rpmSleep)
		select {
		case <-m.cancelCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		pos, err := m.encoder.TicksCount(m.cancelCtx, nil)
		if err != nil {
			m.logger.Info("error getting encoder position, sleeping then continuing: %w", err)
			if !utils.SelectContextOrWait(m.cancelCtx, 100*time.Millisecond) {
				m.logger.Info("error sleeping, giving up %w", m.cancelCtx.Err())
				return
			}
			continue
		}
		now := time.Now().UnixNano()
		if now == lastTime {
			// this really only happens in testing, b/c we decrease sleep, but nice defense anyway
			continue
		}
		atomic.AddInt64(&m.rpmMonitorCalls, 1)
		// TODO: we round down here for absolute encoders, but absolute encoders
		// should have their own logic separate from incremental
		roundedPos := int64(math.Floor(pos))
		m.rpmMonitorPass(roundedPos, lastPos, now, lastTime, rpmDebug)

		lastPos = int64(pos)
		lastTime = now
	}
}

func (m *EncodedMotor) rpmMonitorPass(pos, lastPos, now, lastTime int64, rpmDebug bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	var ticksLeft int64

	currentRPM := m.computeRPM(pos, lastPos, now, lastTime)
	m.state.currentRPM = currentRPM

	if !m.state.regulated && math.Abs(m.state.desiredRPM) > 0.001 {
		m.rpmMonitorPassSetRpmInLock(currentRPM, m.state.desiredRPM, -1, rpmDebug)
		return
	}

	if m.state.regulated {
		// correctly set the ticksLeft accounting for power supplied to the motor and the expected direction of the motor
		ticksLeft = (m.state.setPoint - pos) * sign(m.state.lastPowerPct) * m.flip
		rotationsLeft := float64(ticksLeft) / float64(m.cfg.TicksPerRotation)
		if rpmDebug {
			m.logger.Debugf("ticksLeft %d rotationsLeft %.2f", ticksLeft, rotationsLeft)
		}
		if rotationsLeft <= 0 { // if we have reached goal or overshot, turn off
			if rpmDebug {
				m.logger.Debugf("rot %.2f, stopping motor", rotationsLeft)
			}
			err := m.off(m.cancelCtx)
			if err != nil {
				m.logger.Warnf("error turning motor off from after hit set point: %v", err)
			}
		} else { // halve and quarter rpm values based on seconds remaining in move
			desiredRPM := m.state.desiredRPM
			timeLeftSeconds := 60.0 * rotationsLeft / desiredRPM

			if timeLeftSeconds > 0 {
				if timeLeftSeconds < .5 {
					desiredRPM /= 2
				}
				if timeLeftSeconds < .1 {
					desiredRPM /= 2
				}
			}

			if rpmDebug {
				m.logger.Debugf("rotationsLeft %.2f timeLeft %.2f", rotationsLeft, timeLeftSeconds)
			}

			m.rpmMonitorPassSetRpmInLock(currentRPM, desiredRPM, rotationsLeft, rpmDebug)
		}
	}
}

func (m *EncodedMotor) computeRPM(pos, lastPos, now, lastTime int64) float64 {
	minutes := float64(now-lastTime) / (1e9 * 60)
	if minutes == 0 {
		return 0.0
	}
	rotations := float64(pos-lastPos) / float64(m.cfg.TicksPerRotation)
	return rotations / minutes
}

func (m *EncodedMotor) computeNewPowerPct(currentRPM, desiredRPM float64) float64 {
	lastPowerPct := m.state.lastPowerPct
	if math.Abs(currentRPM) <= 0.001 {
		if math.Abs(lastPowerPct) < 0.01 {
			// We began stopped. Set the power to a low setting so we can get started.
			return .01 * float64(sign(desiredRPM))
		}
		// We've been putting power to the motor, but it's not moving yet. Try increasing the power
		// to it, and we'll start moving soon.
		return m.computeRamp(lastPowerPct, lastPowerPct*2)
	}

	dOverC := desiredRPM / currentRPM * float64(m.flip)
	dOverC = math.Min(dOverC, 2)
	dOverC = math.Max(dOverC, -2)

	// The last power percent resulted in the last RPM measurement. To get to the desired RPM,
	// multiply by their ratio.
	neededPowerPct := lastPowerPct * dOverC

	// Bound neededPowerPct between 0.01 and 1 in the positive or negative direction.
	if !math.Signbit(neededPowerPct) { // neededPowerPct is positive
		neededPowerPct = math.Max(neededPowerPct, 0.01)
		neededPowerPct = math.Min(neededPowerPct, 1)
	} else { // neededPowerPct is negative
		neededPowerPct = math.Min(neededPowerPct, -0.01)
		neededPowerPct = math.Max(neededPowerPct, -1)
	}

	return m.computeRamp(lastPowerPct, neededPowerPct)
}

func (m *EncodedMotor) rpmMonitorPassSetRpmInLock(currentRPM, desiredRPM, rotationsLeft float64, rpmDebug bool) {
	lastPowerPct := m.state.lastPowerPct

	newPowerPct := m.computeNewPowerPct(currentRPM, desiredRPM)
	if newPowerPct == lastPowerPct { // No changes to power are needed right now
		if rpmDebug {
			m.logger.Debugf("newPowerPct %.2f equals lastPowerPct %.2f", newPowerPct, lastPowerPct)
		}
		return
	}

	if rpmDebug {
		m.logger.Debugf("currentRPM: %0.1f desiredRPM: %0.1f lastPowerPct -> newPowerPct: %0.1f -> %0.1f rotations left: %0.1f",
			currentRPM, desiredRPM, lastPowerPct*100, newPowerPct*100, rotationsLeft)
		m.logger.Debugf("setting newPowerPct to %.2f", newPowerPct)
	}

	// Otherwise, we change power to the new computed power percentage
	err := m.setPower(m.cancelCtx, newPowerPct, true)
	if err != nil {
		m.logger.Warnf("rpm regulator cannot set power %s", err)
	}
}

func (m *EncodedMotor) computeRamp(oldPower, newPower float64) float64 {
	newPower = math.Min(newPower, 1.0)
	newPower = math.Max(newPower, -1.0)

	//nolint:ifshort // erd: no clue why this fails
	delta := newPower - oldPower
	if math.Abs(delta) <= 1.1/255.0 {
		return m.fixPowerPct(newPower)
	}
	return m.fixPowerPct(oldPower + (delta * m.rampRate))
}

// GoFor instructs the motor to go in a given direction at the given RPM for a number of given revolutions.
// Both the RPM and the revolutions can be assigned negative values to move in a backwards direction.
// Note: if both are negative the motor will spin in the forward direction.
func (m *EncodedMotor) GoFor(ctx context.Context, rpm, revolutions float64, extra map[string]interface{}) error {
	if rpm == 0 {
		return motor.NewZeroRPMError()
	}

	rpm *= float64(m.flip)

	m.state.lastPowerPct = 0 // clear any leftover state from last time

	ctx, done := m.opMgr.New(ctx)
	defer done()

	if err := m.goForInternal(ctx, rpm, revolutions); err != nil {
		return err
	}

	return m.opMgr.WaitTillNotPowered(ctx, time.Millisecond, m, m.Stop)
}

func (m *EncodedMotor) goForInternal(ctx context.Context, rpm, revolutions float64) error {
	m.RPMMonitorStart()

	var d int64 = 1

	// Backwards
	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d *= -1
	}

	revolutions = math.Abs(revolutions)
	rpm = math.Abs(rpm) * float64(d)

	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	if revolutions == 0 {
		// Moving 0 revolutions is a special value meaning "move forever."
		oldRpm := m.state.desiredRPM
		m.state.desiredRPM = rpm
		if math.Abs(oldRpm) > 0.001 && d == m.directionMovingInLock() {
			return nil
		}
		err := m.setPower(ctx, float64(d)*.06, true) // power of 6% is random
		return err
	}

	numTicks := int64(revolutions * float64(m.cfg.TicksPerRotation))

	pos, err := m.encoder.TicksCount(ctx, nil)
	if err != nil {
		return err
	}
	m.state.setPoint = int64(pos) + d*numTicks*m.flip

	_, rpmDebug := getRPMSleepDebug()
	if rpmDebug {
		m.logger.Debugf("received a goFor with rpm %0.1f, revolutions %0.1f and flip %d", rpm, revolutions, m.flip)
		m.logger.Debugf("setpoint %d", m.state.setPoint)
	}

	m.state.desiredRPM = rpm
	m.state.regulated = true
	isOn, _, err := m.IsPowered(ctx, nil)
	if err != nil {
		return err
	}
	if !isOn {
		// if we're off we start slow, otherwise we just set the desired rpm
		err := m.setPower(ctx, float64(d)*0.03, true)
		if err != nil {
			return err
		}
	}
	return nil
}

// off assumes the state lock is held.
func (m *EncodedMotor) off(ctx context.Context) error {
	m.state.desiredRPM = 0
	m.state.regulated = false
	return m.real.Stop(ctx, nil)
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *EncodedMotor) Stop(ctx context.Context, extra map[string]interface{}) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.off(ctx)
}

// IsMoving returns if the motor is moving or not.
func (m *EncodedMotor) IsMoving(ctx context.Context) (bool, error) {
	return m.real.IsMoving(ctx)
}

// IsPowered returns if the motor is on or not, and the power level it's set to.
func (m *EncodedMotor) IsPowered(ctx context.Context, extra map[string]interface{}) (bool, float64, error) {
	return m.real.IsPowered(ctx, extra)
}

// Close cleanly shuts down the motor.
func (m *EncodedMotor) Close() {
	if m.loop != nil {
		m.loop.Stop()
	}
	m.cancel()
	m.activeBackgroundWorkers.Wait()
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero),
// at a specific speed. Regardless of the directionality of the RPM this function will move the motor
// towards the specified target.
func (m *EncodedMotor) GoTo(ctx context.Context, rpm, targetPosition float64, extra map[string]interface{}) error {
	curPos, err := m.Position(ctx, extra)
	if err != nil {
		return err
	}
	moveDistance := targetPosition - curPos

	return m.GoFor(ctx, math.Abs(rpm), moveDistance, extra)
}

// GoTillStop moves until physically stopped (though with a ten second timeout) or stopFunc() returns true.
func (m *EncodedMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	ctx, done := m.opMgr.New(ctx)
	defer done()

	if err := m.goForInternal(ctx, rpm, 0); err != nil {
		return err
	}
	defer func() {
		if err := m.Stop(ctx, nil); err != nil {
			m.logger.Error("failed to turn off motor")
		}
	}()
	var tries, rpmCount uint

	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}
		if stopFunc != nil && stopFunc(ctx) {
			return nil
		}

		// If we start moving OR just try for too long, good for next phase
		m.stateMu.RLock()
		curRPM := m.state.currentRPM
		m.stateMu.RUnlock()
		if math.Abs(curRPM) >= math.Abs(rpm)/10 {
			rpmCount++
		} else {
			rpmCount = 0
		}
		if rpmCount >= 50 || tries > 200 {
			tries = 0
			rpmCount = 0
			break
		}
		tries++
	}

	for {
		if !utils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("context cancelled during GoTillStop")
		}

		if stopFunc != nil && stopFunc(ctx) {
			return nil
		}

		m.stateMu.RLock()
		curRPM := m.state.currentRPM
		m.stateMu.RUnlock()

		if math.Abs(curRPM) <= math.Abs(rpm)/10 {
			rpmCount++
		} else {
			rpmCount = 0
		}

		if rpmCount >= 50 {
			break
		}

		if tries >= 1000 {
			return errors.New("timed out during GoTillStop")
		}

		tries++
	}
	return nil
}

// ResetZeroPosition sets the current position of the motor specified by the request
// (adjusted by a given offset) to be its new zero position.
func (m *EncodedMotor) ResetZeroPosition(ctx context.Context, offset float64, extra map[string]interface{}) error {
	return m.encoder.Reset(ctx, offset*float64(m.cfg.TicksPerRotation), extra)
}
