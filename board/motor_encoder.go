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
	_rpmDebug   = true
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

	i := b.DigitalInterrupt(mc.Encoder)
	if i == nil {
		return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.Encoder, mc.Name)
	}

	var encoderB DigitalInterrupt
	if mc.EncoderB != "" {
		encoderB = b.DigitalInterrupt(mc.EncoderB)
		if encoderB == nil {
			return nil, errors.Errorf("cannot find encoder (%s) for motor (%s)", mc.EncoderB, mc.Name)
		}
	}

	mm2, err := newEncodedMotorTwoEncoders(mc, m, i, encoderB)
	if err != nil {
		return nil, err
	}
	mm2.logger = logger
	mm2.rpmMonitorStart()

	return mm2, nil
}

func newEncodedMotor(cfg MotorConfig, real Motor, encoder DigitalInterrupt) (*encodedMotor, error) {
	return newEncodedMotorTwoEncoders(cfg, real, encoder, nil)
}

func newEncodedMotorTwoEncoders(cfg MotorConfig, real Motor, encoderA, encoderB DigitalInterrupt) (*encodedMotor, error) {
	cancelCtx, cancel := context.WithCancel(context.Background())
	em := &encodedMotor{
		activeBackgroundWorkers: &sync.WaitGroup{},
		cfg:                     cfg,
		real:                    real,
		encoder:                 encoderA,
		encoderB:                encoderB,
		cancelCtx:               cancelCtx,
		cancel:                  cancel,
		stateMu:                 &sync.RWMutex{},
		startedRPMMonitorMu:     &sync.Mutex{},
		rampRate:                cfg.RampRate,
	}

	if em.rampRate < 0 || em.rampRate > 1 {
		return nil, fmt.Errorf("ramp rate needs to be [0,1) but is %v", em.rampRate)
	}
	if em.rampRate == 0 {
		em.rampRate = 0.3 // Use a conservative value by default.
	}

	return em, nil
}

type encodedMotor struct {
	activeBackgroundWorkers *sync.WaitGroup
	cfg                     MotorConfig
	real                    Motor
	encoder                 DigitalInterrupt
	encoderB                DigitalInterrupt

	stateMu *sync.RWMutex
	state   encodedMotorState

	startedRPMMonitor   bool
	startedRPMMonitorMu *sync.Mutex

	// how fast as we increase power do we do so
	// valid numbers are [0, 1)
	// .01 would ramp very slowly, 1 would ramp instantaneously
	rampRate float32

	rpmMonitorCalls int64
	logger          golog.Logger
	cancelCtx       context.Context
	cancel          func()
}

// encodedMotorState is the core, non-statistical state for the motor.
// Multiple values should be updated atomically at the same time.
type encodedMotorState struct {
	regulated       bool
	desiredRPM      float64 // <= 0 means worker should do nothing
	lastPowerPct    float32
	curDirection    pb.DirectionRelative
	setPoint        int64
	curPosition     int64
	timeLeftSeconds float64
}

func (m *encodedMotor) Position(ctx context.Context) (float64, error) {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return float64(m.state.curPosition) / float64(m.cfg.TicksPerRotation), nil
}

func (m *encodedMotor) rawPosition() int64 {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state.curPosition
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

func fixPowerPct(powerPct float32) float32 {
	if powerPct > 1 {
		powerPct = 1
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
	m.state.lastPowerPct = fixPowerPct(powerPct)
	return m.real.Power(ctx, powerPct)
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
	m.state.lastPowerPct = fixPowerPct(powerPct)
	m.state.curDirection = d
	return m.real.Go(ctx, d, powerPct)
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

func (m *encodedMotor) startRegulatorWorker(onStart func()) {
	if m.encoderB == nil {
		m.startSingleEncoderWorker(onStart)
	} else {
		m.startRotaryEncoderWorker(onStart)
	}
}

func (m *encodedMotor) startSingleEncoderWorker(onStart func()) {
	encoderChannel := make(chan bool)
	m.encoder.AddCallback(encoderChannel)
	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		onStart()
		_, rpmDebug := getRPMSleepDebug()
		for {
			stop := false
			select {
			case <-m.cancelCtx.Done():
				return
			default:
			}

			select {
			case <-m.cancelCtx.Done():
				return
			case <-encoderChannel:
			}

			m.stateMu.Lock()
			if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
				m.state.curPosition++
				stop = m.state.regulated && m.state.curPosition >= m.state.setPoint
			} else if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
				m.state.curPosition--
				stop = m.state.regulated && m.state.curPosition <= m.state.setPoint
			} else if rpmDebug {
				m.logger.Warn("got encoder tick but motor should be off")
			}

			if stop {
				err := m.off(m.cancelCtx)
				if err != nil {
					m.logger.Warnf("error turning motor off from after hit set point: %v", err)
				}
			}
			m.stateMu.Unlock()
		}
	}, m.activeBackgroundWorkers.Done)
}

/*
   picture from https://github.com/joan2937/pigpio/blob/master/EXAMPLES/C/ROTARY_ENCODER/rotary_encoder.c
     1   2     3    4    1    2    3    4     1

             +---------+         +---------+      0
             |         |         |         |
   A         |         |         |         |
             |         |         |         |
   +---------+         +---------+         +----- 1

       +---------+         +---------+            0
       |         |         |         |
   B   |         |         |         |
       |         |         |         |
   ----+         +---------+         +---------+  1

*/
func (m *encodedMotor) startRotaryEncoderWorker(onStart func()) {
	chanA := make(chan bool)
	chanB := make(chan bool)

	m.encoder.AddCallback(chanA)
	m.encoderB.AddCallback(chanB)

	m.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		onStart()
		aLevel := true
		bLevel := true

		lastWasA := true
		lastLevel := true

		for {

			select {
			case <-m.cancelCtx.Done():
				return
			default:
			}

			var level bool
			var isA bool

			select {
			case <-m.cancelCtx.Done():
				return
			case level = <-chanA:
				isA = true
				aLevel = level
			case level = <-chanB:
				isA = false
				bLevel = level
			}

			if isA == lastWasA && level == lastLevel {
				// this means we got the exact same message multiple times
				// this is probably some sort of hardware issue, so we ignore
				continue
			}
			lastWasA = isA
			lastLevel = level

			m.stateMu.Lock()

			if !aLevel && !bLevel { // state 1
				if lastWasA {
					m.state.curPosition++
				} else {
					m.state.curPosition--
				}
			} else if !aLevel && bLevel { // state 2
				if lastWasA {
					m.state.curPosition--
				} else {
					m.state.curPosition++
				}
			} else if aLevel && bLevel { // state 3
				if lastWasA {
					m.state.curPosition++
				} else {
					m.state.curPosition--
				}
			} else if aLevel && !bLevel { // state 4
				if lastWasA {
					m.state.curPosition--
				} else {
					m.state.curPosition++
				}
			}

			if m.state.regulated {
				var ticksLeft int64

				if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
					ticksLeft = m.state.setPoint - m.state.curPosition
				} else if m.state.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
					ticksLeft = m.state.curPosition - m.state.setPoint
				}
				rotationsLeft := float64(ticksLeft) / float64(m.cfg.TicksPerRotation)

				stop := rotationsLeft <= 0.0

				if stop {
					m.state.timeLeftSeconds = 0
					err := m.off(m.cancelCtx)
					if err != nil {
						m.logger.Warnf("error turning motor off from after hit set point: %v", err)
					}
				} else {
					m.state.timeLeftSeconds = 60.0 * rotationsLeft / m.state.desiredRPM
				}

			}
			m.stateMu.Unlock()

		}
	}, m.activeBackgroundWorkers.Done)
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
	m.startRegulatorWorker(onStart)

	lastCount := m.encoder.Value()
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

		count := m.encoder.Value()
		now := time.Now().UnixNano()
		if now == lastTime {
			// this really only happens in testing, b/c we decrease sleep, but nice defense anyway
			continue
		}
		atomic.AddInt64(&m.rpmMonitorCalls, 1)

		m.stateMu.Lock()
		desiredRPM := m.state.desiredRPM
		if m.state.timeLeftSeconds > 0 {
			if m.state.timeLeftSeconds < .5 {
				desiredRPM = desiredRPM / 2
			}
			if m.state.timeLeftSeconds < .1 {
				desiredRPM = desiredRPM / 2
			}
		}
		lastPowerPct := m.state.lastPowerPct

		if desiredRPM > 0 {
			rotations := float64(count-lastCount) / float64(m.cfg.TicksPerRotation)
			minutes := float64(now-lastTime) / (1e9 * 60)
			currentRPM := rotations / minutes
			if minutes == 0 {
				currentRPM = 0
			}

			var newPowerPct float32

			if currentRPM == 0 {
				newPowerPct = m.computeRamp(lastPowerPct, 1)
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
					m.logger.Debugf("current rpm: %0.1f powerPct: %v newPowerPct: %v desiredRPM: %0.1f",
						currentRPM, lastPowerPct*100, newPowerPct*100, desiredRPM)
				}
				err := m.setPower(m.cancelCtx, newPowerPct, true)
				if err != nil {
					m.logger.Warnf("rpm regulator cannot set power %s", err)
				}
			}
		}
		m.stateMu.Unlock()

		lastCount = count
		lastTime = now
	}
}

func (m encodedMotor) computeRamp(oldPower, newPower float32) float32 {
	delta := newPower - oldPower
	if math.Abs(float64(delta)) <= 1.0/255.0 {
		return newPower
	}
	return oldPower + (delta * m.rampRate)
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

	m.stateMu.Lock()
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		m.state.setPoint = m.state.curPosition + numTicks
	} else if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		m.state.setPoint = m.state.curPosition - numTicks
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
