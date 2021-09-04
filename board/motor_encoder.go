package board

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.viam.com/utils"

	pb "go.viam.com/core/proto/api/v1"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
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

func setRPMSleepDebug(dur time.Duration, debug bool) func() {
	_rpmDebugMu.Lock()
	prevRPMSleep := _rpmSleep
	prevRPMDebug := _rpmDebug
	_rpmSleep = dur
	_rpmDebug = debug
	_rpmDebugMu.Unlock()
	return func() {
		setRPMSleepDebug(prevRPMSleep, prevRPMDebug)
	}
}

// WrapMotorWithEncoder takes a motor and adds an encoder onto it in order to understand its odometry.
func WrapMotorWithEncoder(ctx context.Context, b Board, mc MotorConfig, m Motor, logger golog.Logger) (Motor, error) {
	if mc.Encoder == "" {
		return m, nil
	}

	if mc.TicksPerRotation == 0 {
		return nil, errors.Errorf("need a TicksPerRotation for motor (%s)", mc.Name)
	}

	i, ok := b.DigitalInterruptByName(mc.Encoder)
	if !ok {
		return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.Encoder, mc.Name)
	}

	var mm *encodedMotor
	var err error

	if mc.EncoderB == "" {
		encoder := &singleEncoder{i: i}
		mm, err = newEncodedMotor(mc, m, encoder, logger)
		if err != nil {
			return nil, err
		}
		encoder.m = mm
	} else {
		b, ok := b.DigitalInterruptByName(mc.EncoderB)
		if !ok {
			return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.EncoderB, mc.Name)
		}
		mm, err = newEncodedMotor(mc, m, NewHallEncoder(i, b), logger)
		if err != nil {
			return nil, err
		}
	}

	mm.rpmMonitorStart()

	return mm, nil
}

// NewEncodedMotor creates a new motor that supports an arbitrary source of encoder information
func NewEncodedMotor(cfg MotorConfig, real Motor, encoder Encoder, logger golog.Logger) (Motor, error) {
	return newEncodedMotor(cfg, real, encoder, logger)
}

func newEncodedMotor(cfg MotorConfig, real Motor, encoder Encoder, logger golog.Logger) (*encodedMotor, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	em := &encodedMotor{
		activeBackgroundWorkers: &sync.WaitGroup{},
		cfg:                     cfg,
		real:                    real,
		encoder:                 encoder,
		cancelCtx:               cancelCtx,
		cancel:                  cancel,
		stateMu:                 &sync.RWMutex{},
		startedRPMMonitorMu:     &sync.Mutex{},
		rampRate:                cfg.RampRate,
		maxPowerPct:             cfg.MaxPowerPct,
		logger:                  logger,
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

	if val, ok := cfg.Attributes["rpmDebug"]; ok {
		if val == "true" {
			_rpmDebug = true
		}
	}

	return em, nil
}

type encodedMotor struct {
	activeBackgroundWorkers *sync.WaitGroup
	cfg                     MotorConfig
	real                    Motor
	encoder                 Encoder

	stateMu *sync.RWMutex
	state   encodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu *sync.Mutex

	// how fast as we increase power do we do so
	// valid numbers are [0, 1)
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate    float32
	maxPowerPct float32

	rpmMonitorCalls int64
	logger          golog.Logger
	cancelCtx       context.Context
	cancel          func()
}

// encodedMotorState is the core, non-statistical state for the motor.
// Multiple values should be updated atomically at the same time.
type encodedMotorState struct {
	regulated    bool
	desiredRPM   float64 // <= 0 means worker should do nothing
	currentRPM   float64
	lastPowerPct float32
	curDirection pb.DirectionRelative
	setPoint     int64
}

func (m *encodedMotor) Position(ctx context.Context) (float64, error) {
	ticks, err := m.encoder.Position(ctx)
	if err != nil {
		return 0, err
	}
	return float64(ticks) / float64(m.cfg.TicksPerRotation), nil
}

func (m *encodedMotor) rawDirection() pb.DirectionRelative {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state.curDirection
}

func (m *encodedMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *encodedMotor) RPMMonitorCalls() int64 {
	return atomic.LoadInt64(&m.rpmMonitorCalls)
}

func (m *encodedMotor) isRegulated() bool {
	m.stateMu.RLock()
	regulated := m.state.regulated
	m.stateMu.RUnlock()
	return regulated
}

func (m *encodedMotor) setRegulated(b bool) {
	m.stateMu.Lock()
	m.state.regulated = b
	m.stateMu.Unlock()
}

func (m *encodedMotor) fixPowerPct(powerPct float32) float32 {
	if powerPct > m.maxPowerPct {
		powerPct = m.maxPowerPct
	} else if powerPct < 0 {
		powerPct = 0
	}
	return powerPct
}

func (m *encodedMotor) Power(ctx context.Context, powerPct float32) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.setPower(ctx, powerPct, false)
}

// setPower assumes the state lock is held
func (m *encodedMotor) setPower(ctx context.Context, powerPct float32, internal bool) error {
	if !internal {
		m.state.desiredRPM = 0 // if we're setting power externally, don't control RPM
	}
	m.state.lastPowerPct = m.fixPowerPct(powerPct)
	return m.real.Power(ctx, m.state.lastPowerPct)
}

func (m *encodedMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.doGo(ctx, d, powerPct, false)
}

// doGo assumes the state lock is held
func (m *encodedMotor) doGo(ctx context.Context, d pb.DirectionRelative, powerPct float32, internal bool) error {
	if !internal {
		m.state.desiredRPM = 0    // if we're setting power externally, don't control RPM
		m.state.regulated = false // user wants direct control, so we stop trying to control the world
	}
	m.state.lastPowerPct = m.fixPowerPct(powerPct)
	m.state.curDirection = d
	return m.real.Go(ctx, d, m.state.lastPowerPct)
}

func (m *encodedMotor) rpmMonitorStart() {
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

func (m *encodedMotor) rpmMonitor(onStart func()) {
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

	lastPos, err := m.encoder.Position(m.cancelCtx)
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

		pos, err := m.encoder.Position(m.cancelCtx)
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

func (m *encodedMotor) rpmMonitorPass(pos, lastPos, now, lastTime int64, rpmDebug bool) {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()

	var ticksLeft int64

	if !m.state.regulated && m.state.desiredRPM > 0 {
		m.rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime, m.state.desiredRPM, -1, rpmDebug)
		return
	}

	if m.state.regulated {
		if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
			ticksLeft = m.state.setPoint - pos
		} else if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
			ticksLeft = pos - m.state.setPoint
		}

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
					desiredRPM = desiredRPM / 2
				}
				if timeLeftSeconds < .1 {
					desiredRPM = desiredRPM / 2
				}
			}
			m.rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime, desiredRPM, rotationsLeft, rpmDebug)
		}
	}

}

func (m *encodedMotor) rpmMonitorPassSetRpmInLock(pos, lastPos, now, lastTime int64, desiredRPM, rotationsLeft float64, rpmDebug bool) {
	lastPowerPct := m.state.lastPowerPct

	rotations := float64(pos-lastPos) / float64(m.cfg.TicksPerRotation)
	minutes := float64(now-lastTime) / (1e9 * 60)
	currentRPM := math.Abs(rotations / minutes)
	if minutes == 0 {
		currentRPM = 0
	}
	m.state.currentRPM = currentRPM

	var newPowerPct float32

	if currentRPM == 0 {
		if lastPowerPct < .01 {
			newPowerPct = .01
		} else {
			newPowerPct = m.computeRamp(lastPowerPct, lastPowerPct*2)
		}
	} else {
		dOverC := desiredRPM / currentRPM
		if dOverC > 2 {
			dOverC = 2
		}
		neededPowerPct := float64(lastPowerPct) * dOverC

		if neededPowerPct < .01 {
			neededPowerPct = .01
		} else if neededPowerPct > 1 {
			neededPowerPct = 1
		}

		newPowerPct = m.computeRamp(lastPowerPct, float32(neededPowerPct))
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

func (m encodedMotor) computeRamp(oldPower, newPower float32) float32 {
	if newPower > 1.0 {
		newPower = 1.0
	}
	delta := newPower - oldPower
	if math.Abs(float64(delta)) <= 1.0/255.0 {
		return m.fixPowerPct(newPower)
	}
	return m.fixPowerPct(oldPower + (delta * m.rampRate))
}

func (m *encodedMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return m.Off(ctx)
	}

	m.rpmMonitorStart()

	if revolutions < 0 {
		revolutions *= -1
		d = FlipDirection(d)
	}

	if revolutions == 0 {
		m.stateMu.Lock()
		oldRpm := m.state.desiredRPM
		curDirection := m.state.curDirection
		m.state.desiredRPM = rpm
		if oldRpm > 0 && d == curDirection {
			m.stateMu.Unlock()
			return nil
		}
		err := m.doGo(ctx, d, .06, true) // power of 6% is random
		m.stateMu.Unlock()
		return err
	}

	numTicks := int64(revolutions * float64(m.cfg.TicksPerRotation))

	pos, err := m.encoder.Position(ctx)
	if err != nil {
		return err
	}
	m.stateMu.Lock()
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		m.state.setPoint = pos + numTicks
	} else if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		m.state.setPoint = pos - numTicks
	} else {
		m.stateMu.Unlock()
		panic("impossible")
	}

	m.state.desiredRPM = rpm
	m.state.regulated = true

	isOn, err := m.IsOn(ctx)
	if err != nil {
		m.stateMu.Unlock()
		return err
	}
	if !isOn {
		// if we're off we start slow, otherwise we just set the desired rpm
		err := m.doGo(ctx, d, .03, true)
		if err != nil {
			m.stateMu.Unlock()
			return err
		}
	}
	m.stateMu.Unlock()

	return nil
}

// off assumes the state lock is held
func (m *encodedMotor) off(ctx context.Context) error {
	m.state.desiredRPM = 0
	m.state.regulated = false
	return m.real.Off(ctx)
}

func (m *encodedMotor) Off(ctx context.Context) error {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	return m.off(ctx)
}

func (m *encodedMotor) IsOn(ctx context.Context) (bool, error) {
	return m.real.IsOn(ctx)
}

func (m *encodedMotor) Close() error {
	m.cancel()
	m.activeBackgroundWorkers.Wait()
	return nil
}

// GoTo instructs the motor to go to a specific position (provided in revolutions from home/zero), at a specific speed.
func (m *encodedMotor) GoTo(ctx context.Context, rpm float64, targetPosition float64) error {
	curPos, err := m.Position(ctx)
	if err != nil {
		return err
	}
	dir := pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD
	moveDistance := targetPosition - curPos
	if math.Signbit(moveDistance) {
		dir = pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD
		moveDistance = math.Abs(moveDistance)
	}

	return m.GoFor(ctx, dir, rpm, moveDistance)
}

// GoTillStop moves until physically stopped (though with a ten second timeout) or stopFunc() returns true.
func (m *encodedMotor) GoTillStop(ctx context.Context, d pb.DirectionRelative, rpm float64, stopFunc func(ctx context.Context) bool) error {
	if err := m.GoFor(ctx, d, rpm, 0); err != nil {
		return err
	}
	defer func() {
		if err := m.Off(ctx); err != nil {
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
		if curRPM >= rpm/10 {
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

		if curRPM <= rpm/10 {
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

// Zero resets the position to zero/home
func (m *encodedMotor) Zero(ctx context.Context, offset float64) error {
	return m.encoder.Zero(ctx, int64(offset*float64(m.cfg.TicksPerRotation)))
}
