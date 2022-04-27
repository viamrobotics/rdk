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

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/motor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/control"
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
	prevRPMSleep := _rpmSleep
	prevRPMDebug := _rpmDebug
	_rpmSleep = dur
	_rpmDebug = debug
	_rpmDebugMu.Unlock()
	return func() {
		SetRPMSleepDebug(prevRPMSleep, prevRPMDebug)
	}
}

// WrapMotorWithEncoder takes a motor and adds an encoder onto it in order to understand its odometry.
func WrapMotorWithEncoder(
	ctx context.Context,
	encoderBoard board.Board,
	c config.Component,
	mc motor.Config,
	m motor.Motor,
	logger golog.Logger,
) (motor.Motor, error) {
	if mc.EncoderA == "" {
		return m, nil
	}

	if mc.TicksPerRotation == 0 {
		return nil, errors.Errorf("need a TicksPerRotation for motor (%s)", c.Name)
	}

	i, ok := encoderBoard.DigitalInterruptByName(mc.EncoderA)
	if !ok {
		return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.EncoderA, c.Name)
	}

	var mm *EncodedMotor
	var err error

	if mc.EncoderB == "" {
		encoder := board.NewSingleEncoder(i, mm)
		mm, err = newEncodedMotor(c, mc, m, encoder, logger)
		if err != nil {
			return nil, err
		}

		// Adds encoded motor to encoder
		encoder.AttachDirectionalAwareness(mm)
	} else {
		b, ok := encoderBoard.DigitalInterruptByName(mc.EncoderB)
		if !ok {
			return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.EncoderB, c.Name)
		}
		mm, err = newEncodedMotor(c, mc, m, board.NewHallEncoder(i, b), logger)
		if err != nil {
			return nil, err
		}
	}

	mm.RPMMonitorStart()

	return mm, nil
}

// NewEncodedMotor creates a new motor that supports an arbitrary source of encoder information.
func NewEncodedMotor(
	config config.Component,
	motorConfig motor.Config,
	realMotor motor.Motor,
	encoder board.Encoder,
	logger golog.Logger,
) (motor.Motor, error) {
	return newEncodedMotor(config, motorConfig, realMotor, encoder, logger)
}

func newEncodedMotor(
	config config.Component,
	motorConfig motor.Config,
	realMotor motor.Motor,
	encoder board.Encoder,
	logger golog.Logger,
) (*EncodedMotor, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	em := &EncodedMotor{
		activeBackgroundWorkers: &sync.WaitGroup{},
		cfg:                     motorConfig,
		real:                    realMotor,
		encoder:                 encoder,
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
		cLoop, err := control.NewControlLoop(logger, motorConfig.ControlLoop, em)
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
		return nil, fmt.Errorf("ramp rate needs to be [0,1) but is %v", em.rampRate)
	}
	if em.rampRate == 0 {
		em.rampRate = 0.2 // Use a conservative value by default.
	}

	if em.maxPowerPct < 0 || em.maxPowerPct > 1 {
		return nil, fmt.Errorf("max power pct needs to be [0,1) but is %v", em.maxPowerPct)
	}
	if em.maxPowerPct == 0 {
		em.maxPowerPct = 1.0
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
	cfg                     motor.Config
	real                    motor.Motor
	encoder                 board.Encoder

	stateMu *sync.RWMutex
	state   EncodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu *sync.Mutex

	// how fast as we increase power do we do so
	// valid numbers are [0, 1)
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate    float64
	maxPowerPct float64

	rpmMonitorCalls int64
	logger          golog.Logger
	cancelCtx       context.Context
	cancel          func()
	loop            *control.ControlLoop
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

// GetPosition returns the position of the motor.
func (m *EncodedMotor) GetPosition(ctx context.Context) (float64, error) {
	ticks, err := m.encoder.GetPosition(ctx)
	if err != nil {
		return 0, err
	}
	return float64(ticks) / float64(m.cfg.TicksPerRotation), nil
}

// DirectionMoving returns the direction we are currently mpving in, with 1 representing
// forward and  -1 representing backwards.
func (m *EncodedMotor) DirectionMoving() int64 {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.directionMovingInLock()
}

func (m *EncodedMotor) directionMovingInLock() int64 {
	if !math.Signbit(m.state.lastPowerPct) {
		return 1
	}

	return -1
}

// GetFeatures returns the status of whether the motor supports certain optional features.
func (m *EncodedMotor) GetFeatures(ctx context.Context) (map[motor.Feature]bool, error) {
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
	m.state.regulated = b
	m.stateMu.Unlock()
}

func (m *EncodedMotor) fixPowerPct(powerPct float64) float64 {
	powerPct = math.Min(powerPct, m.maxPowerPct)
	powerPct = math.Max(powerPct, -1*m.maxPowerPct)
	return powerPct
}

// SetPower sets the power of the motor to the given percentage value between 0 and 1.
func (m *EncodedMotor) SetPower(ctx context.Context, powerPct float64) error {
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
	return m.real.SetPower(ctx, m.state.lastPowerPct)
}

// RPMMonitorStart starts the RPM monitor.
func (m *EncodedMotor) RPMMonitorStart() {
	m.startedRPMMonitorMu.Lock()
	startedRPMMonitor := m.startedRPMMonitor
	m.startedRPMMonitorMu.Unlock()
	if startedRPMMonitor {
		return
	}
	started := make(chan struct{})
	m.activeBackgroundWorkers.Add(1)
	var closeOnce bool
	utils.ManagedGo(func() {
		m.rpmMonitor(func() {
			if !closeOnce {
				closeOnce = true
				close(started)
			}
		})
	}, m.activeBackgroundWorkers.Done)
	<-started
}

func (m *EncodedMotor) rpmMonitor(onStart func()) {
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

	// just a convenient place to start the encoder listener
	m.encoder.Start(m.cancelCtx, m.activeBackgroundWorkers, onStart)

	lastPos, err := m.encoder.GetPosition(m.cancelCtx)
	if err != nil {
		panic(err)
	}
	lastTime := time.Now().UnixNano()

	rpmSleep, rpmDebug := getRPMSleepDebug()
	for {
		select {
		case <-m.cancelCtx.Done():
			return
		default:
		}

		timer := time.NewTimer(rpmSleep)
		select {
		case <-m.cancelCtx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		pos, err := m.encoder.GetPosition(m.cancelCtx)
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

		m.rpmMonitorPass(pos, lastPos, now, lastTime, rpmDebug)

		lastPos = pos
		lastTime = now
	}
}

func (m *EncodedMotor) rpmMonitorPass(pos, lastPos, now, lastTime int64, rpmDebug bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	var ticksLeft int64

	if !m.state.regulated && math.Abs(m.state.desiredRPM) > 0.001 {
		m.rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime, m.state.desiredRPM, -1, rpmDebug)
		return
	}

	if m.state.regulated {
		ticksLeft = (m.state.setPoint - pos) * int64(m.state.lastPowerPct/math.Abs(m.state.lastPowerPct))
		rotationsLeft := float64(ticksLeft) / float64(m.cfg.TicksPerRotation)
		if rotationsLeft <= 0 {
			err := m.off(m.cancelCtx)
			if err != nil {
				m.logger.Warnf("error turning motor off from after hit set point: %v", err)
			}
		} else {
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
			m.rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime, desiredRPM, rotationsLeft, rpmDebug)
		}
	}
}

func (m *EncodedMotor) rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime int64, desiredRPM, rotationsLeft float64, rpmDebug bool) {
	lastPowerPct := m.state.lastPowerPct

	rotations := float64(pos-lastPos) / float64(m.cfg.TicksPerRotation)
	minutes := float64(now-lastTime) / (1e9 * 60)
	currentRPM := rotations / minutes
	if minutes == 0 {
		currentRPM = 0
	}
	m.state.currentRPM = currentRPM

	var newPowerPct float64

	if math.Abs(currentRPM) <= 0.001 {
		if math.Abs(lastPowerPct) < 0.01 {
			newPowerPct = .01 * desiredRPM / math.Abs(desiredRPM)
		} else {
			newPowerPct = m.computeRamp(lastPowerPct, lastPowerPct*2)
		}
	} else {
		dOverC := desiredRPM / currentRPM
		dOverC = math.Min(dOverC, 2)
		dOverC = math.Max(dOverC, -2)

		neededPowerPct := lastPowerPct * dOverC

		if !math.Signbit(neededPowerPct) {
			neededPowerPct = math.Max(neededPowerPct, 0.01)
			neededPowerPct = math.Min(neededPowerPct, 1)
		} else {
			neededPowerPct = math.Min(neededPowerPct, -0.01)
			neededPowerPct = math.Max(neededPowerPct, -1)
		}

		newPowerPct = m.computeRamp(lastPowerPct, neededPowerPct)
	}

	if newPowerPct != lastPowerPct {
		if rpmDebug {
			m.logger.Debugf("current rpm: %0.1f desiredRPM: %0.1f power: %0.1f -> %0.1f rot2go: %0.1f",
				currentRPM, desiredRPM, lastPowerPct*100, newPowerPct*100, rotationsLeft)
		}
		err := m.setPower(m.cancelCtx, newPowerPct, true)
		if err != nil {
			m.logger.Warnf("rpm regulator cannot set power %s", err)
		}
	}
}

func (m *EncodedMotor) computeRamp(oldPower, newPower float64) float64 {
	newPower = math.Min(newPower, 1.0)
	newPower = math.Max(newPower, -1.0)

	delta := newPower - oldPower
	if math.Abs(delta) <= 1.1/255.0 {
		return m.fixPowerPct(newPower)
	}
	return m.fixPowerPct(oldPower + (delta * m.rampRate))
}

// GoFor instructs the motor to go in a given direction at the given RPM for a number of given revolutions.
// Both the RPM and the revolutions can be assigned negative values to move in a backwards direction.
// Note: if both are negative the motor will spin in the forward direction.
func (m *EncodedMotor) GoFor(ctx context.Context, rpm float64, revolutions float64) error {
	m.RPMMonitorStart()

	var d int64 = 1

	// Backwards
	if math.Signbit(revolutions) != math.Signbit(rpm) {
		d *= -1
	}

	revolutions = math.Abs(revolutions)
	rpm = math.Abs(rpm) * float64(d)

	if revolutions == 0 {
		m.stateMu.Lock()
		oldRpm := m.state.desiredRPM
		m.state.desiredRPM = rpm
		if math.Abs(oldRpm) > 0.001 && d == m.directionMovingInLock() {
			m.stateMu.Unlock()
			return nil
		}
		err := m.setPower(ctx, float64(d)*.06, true) // power of 6% is random
		m.stateMu.Unlock()
		return err
	}

	numTicks := int64(revolutions * float64(m.cfg.TicksPerRotation))

	pos, err := m.encoder.GetPosition(ctx)
	if err != nil {
		return err
	}

	m.stateMu.Lock()
	if d == 1 || d == -1 {
		m.state.setPoint = pos + d*numTicks
	} else {
		m.stateMu.Unlock()
		panic("impossible")
	}

	m.state.desiredRPM = rpm
	m.state.regulated = true
	isOn, err := m.IsPowered(ctx)
	if err != nil {
		m.stateMu.Unlock()
		return err
	}
	if !isOn {
		// if we're off we start slow, otherwise we just set the desired rpm
		err := m.setPower(ctx, float64(d)*0.03, true)
		if err != nil {
			m.stateMu.Unlock()
			return err
		}
	}
	m.stateMu.Unlock()
	return nil
}

// off assumes the state lock is held.
func (m *EncodedMotor) off(ctx context.Context) error {
	m.state.desiredRPM = 0
	m.state.regulated = false
	return m.real.Stop(ctx)
}

// Stop turns the power to the motor off immediately, without any gradual step down.
func (m *EncodedMotor) Stop(ctx context.Context) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.off(ctx)
}

// IsPowered returns if the motor is on or not.
func (m *EncodedMotor) IsPowered(ctx context.Context) (bool, error) {
	return m.real.IsPowered(ctx)
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
func (m *EncodedMotor) GoTo(ctx context.Context, rpm float64, targetPosition float64) error {
	curPos, err := m.GetPosition(ctx)
	if err != nil {
		return err
	}
	moveDistance := targetPosition - curPos

	return m.GoFor(ctx, math.Abs(rpm), moveDistance)
}

// GoTillStop moves until physically stopped (though with a ten second timeout) or stopFunc() returns true.
func (m *EncodedMotor) GoTillStop(ctx context.Context, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if err := m.GoFor(ctx, rpm, 0); err != nil {
		return err
	}
	defer func() {
		if err := m.Stop(ctx); err != nil {
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
func (m *EncodedMotor) ResetZeroPosition(ctx context.Context, offset float64) error {
	return m.encoder.ResetZeroPosition(ctx, int64(offset*float64(m.cfg.TicksPerRotation)))
}
