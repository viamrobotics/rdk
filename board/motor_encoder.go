package board

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	pb "go.viam.com/robotcore/proto/api/v1"

	"github.com/edaniels/golog"
)

var (
	rpmSleep = 50 * time.Millisecond // really just for testing
	rpmDebug = false
)

func WrapMotorWithEncoder(ctx context.Context, b Board, mc MotorConfig, m Motor, logger golog.Logger) (Motor, error) {
	if mc.Encoder == "" {
		return m, nil
	}

	if mc.TicksPerRotation == 0 {
		return nil, fmt.Errorf("need a TicksPerRotation for motor (%s)", mc.Name)
	}

	i := b.DigitalInterrupt(mc.Encoder)
	if i == nil {
		return nil, fmt.Errorf("cannot find encoder (%s) for motor (%s)", mc.Encoder, mc.Name)
	}

	var encoderB DigitalInterrupt
	if mc.EncoderB != "" {
		encoderB = b.DigitalInterrupt(mc.EncoderB)
		if encoderB == nil {
			return nil, fmt.Errorf("cannot find encoder (%s) for motor (%s)", mc.EncoderB, mc.Name)
		}
	}

	mm2 := &encodedMotor{
		cfg:      mc,
		real:     m,
		encoder:  i,
		encoderB: encoderB,
		logger:   logger,
	}
	mm2.rpmMonitorStart(ctx)

	return mm2, nil
}

type encodedMotor struct {
	cfg      MotorConfig
	real     Motor
	encoder  DigitalInterrupt
	encoderB DigitalInterrupt

	regulated int32 // use atomic operations when access

	// TODO(erh): check thread safety on this
	desiredRPM float64 // <= 0 means thread should do nothing

	lastPowerPct float32
	curDirection pb.DirectionRelative
	setPoint     int64

	curPosition int64

	startedRPMMonitor bool
	rpmMonitorCalls   int64
	logger            golog.Logger
}

func (m *encodedMotor) Position(ctx context.Context) (float64, error) {
	return float64(m.curPosition) / float64(m.cfg.TicksPerRotation), nil
}

func (m *encodedMotor) PositionSupported(ctx context.Context) (bool, error) {
	return true, nil
}

func (m *encodedMotor) isRegulated() bool {
	return atomic.LoadInt32(&m.regulated) == 1
}

func (m *encodedMotor) setRegulated(b bool) {
	if b {
		atomic.StoreInt32(&m.regulated, 1)
	} else {
		atomic.StoreInt32(&m.regulated, 0)
	}
}

func (m *encodedMotor) Power(ctx context.Context, powerPct float32) error {
	m.desiredRPM = 0 // if we're setting power manually, don't control RPM
	return m.setPower(ctx, powerPct)
}

func (m *encodedMotor) setPower(ctx context.Context, powerPct float32) error {
	if powerPct > 1 {
		powerPct = 1
	} else if powerPct < 0 {
		powerPct = 0
	}
	m.lastPowerPct = powerPct
	return m.real.Power(ctx, powerPct)
}

func (m *encodedMotor) Go(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.setRegulated(false) // user wants direct control, so we stop trying to control the world
	m.desiredRPM = 0      // if we're setting power manually, don't control RPM
	return m.doGo(ctx, d, powerPct)
}

func (m *encodedMotor) doGo(ctx context.Context, d pb.DirectionRelative, powerPct float32) error {
	m.lastPowerPct = powerPct
	m.curDirection = d
	return m.real.Go(ctx, d, powerPct)
}

func (m *encodedMotor) rpmMonitorStart(ctx context.Context) {
	if m.startedRPMMonitor {
		return
	}
	go m.rpmMonitor(ctx)
}

func (m *encodedMotor) startRegulatorThread(ctx context.Context) {
	if m.encoderB == nil {
		m.startSingleEncoderThread(ctx)
	} else {
		m.startRotaryEncoderThread(ctx)
	}
}

func (m *encodedMotor) startSingleEncoderThread(ctx context.Context) {
	encoderChannel := make(chan bool)
	m.encoder.AddCallback(encoderChannel)
	go func() {
		for {
			stop := false

			<-encoderChannel

			if m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
				m.curPosition++
				stop = m.isRegulated() && m.curPosition >= m.setPoint
			} else if m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
				m.curPosition--
				stop = m.isRegulated() && m.curPosition <= m.setPoint
			} else if rpmDebug {
				m.logger.Warnf("got encoder tick but motor should be off")
			}

			if stop {
				err := m.Off(ctx)
				if err != nil {
					m.logger.Warnf("error turning motor off from after hit set point: %v", err)
				}
				m.setRegulated(false)
			}
		}
	}()
}

func (m *encodedMotor) startRotaryEncoderThread(ctx context.Context) {
	chanA := make(chan bool)
	chanB := make(chan bool)

	m.encoder.AddCallback(chanA)
	m.encoderB.AddCallback(chanB)

	go func() {
		aLevel := true
		bLevel := true

		lastWasA := true

		for {

			var level bool
			var isA bool

			select {
			case level = <-chanA:
				isA = true
				aLevel = level
			case level = <-chanB:
				isA = false
				bLevel = level
			}

			//fmt.Printf("isA: %v level: %v aLevel: %v bLevel: %v lastWasA: %v\n", isA, level, aLevel, bLevel, lastWasA)

			if isA == lastWasA {
				lastWasA = isA
				continue
			}
			lastWasA = isA

			if isA && level {
				if bLevel {
					m.curPosition++
				}
			} else if !isA && level {
				if aLevel {
					m.curPosition--
				}
			}

			if m.isRegulated() {
				stop := (m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD && m.curPosition >= m.setPoint) ||
					(m.curDirection == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD && m.curPosition <= m.setPoint)

				if stop {
					err := m.Off(ctx)
					if err != nil {
						m.logger.Warnf("error turning motor off from after hit set point: %v", err)
					}
					m.setRegulated(false)
				}
			}
		}
	}()

}

func (m *encodedMotor) rpmMonitor(ctx context.Context) {
	if m.encoder == nil {
		panic(fmt.Errorf("started rpmMonitor but have no encoder"))
	}

	if m.startedRPMMonitor {
		return
	}
	m.startedRPMMonitor = true

	// just a convenient place to start the encoder listener
	m.startRegulatorThread(ctx)

	lastCount := m.encoder.Value()
	lastTime := time.Now().UnixNano()

	for {

		time.Sleep(rpmSleep)

		count := m.encoder.Value()
		now := time.Now().UnixNano()
		if now == lastTime {
			// this really only happens in testing, b/c we decrease sleep, but nice defense anyway
			continue
		}
		m.rpmMonitorCalls++

		if m.desiredRPM > 0 {
			rotations := float64(count-lastCount) / float64(m.cfg.TicksPerRotation)
			minutes := float64(now-lastTime) / (1e9 * 60)
			currentRPM := rotations / minutes
			if minutes == 0 {
				currentRPM = 0
			}

			var newPowerPct float32

			if currentRPM == 0 {
				newPowerPct = m.lastPowerPct + 0.0625
				if newPowerPct > 1 {
					newPowerPct = 1
				}
			} else {
				dOverC := m.desiredRPM / currentRPM
				if dOverC > 2 {
					dOverC = 2
				}
				neededPowerPct := float64(m.lastPowerPct) * dOverC

				if neededPowerPct < .01 {
					neededPowerPct = .01
				} else if neededPowerPct > 1 {
					neededPowerPct = 1
				}

				neededPowerPct = (float64(m.lastPowerPct) + neededPowerPct) / 2 // slow down ramps

				newPowerPct = float32(neededPowerPct)
			}

			if newPowerPct != m.lastPowerPct {
				if rpmDebug {
					m.logger.Debugf("current rpm: %0.1f powerPct: %v newPowerPct: %v desiredRPM: %0.1f",
						currentRPM, m.lastPowerPct*100, newPowerPct*100, m.desiredRPM)
				}
				err := m.setPower(ctx, newPowerPct)
				if err != nil {
					m.logger.Warnf("rpm regulator cannot set power %s", err)
				}
			}
		}

		lastCount = count
		lastTime = now
	}
}

func (m *encodedMotor) GoFor(ctx context.Context, d pb.DirectionRelative, rpm float64, revolutions float64) error {
	if d == pb.DirectionRelative_DIRECTION_RELATIVE_UNSPECIFIED {
		return m.Off(ctx)
	}

	m.rpmMonitorStart(ctx)

	if revolutions < 0 {
		revolutions *= -1
		d = FlipDirection(d)
	}

	if revolutions == 0 {
		oldRpm := m.desiredRPM
		m.desiredRPM = rpm
		if oldRpm > 0 && d == m.curDirection {
			return nil
		}
		return m.doGo(ctx, d, .06) // power of 6% is random
	}

	numTicks := int64(revolutions * float64(m.cfg.TicksPerRotation))

	if d == pb.DirectionRelative_DIRECTION_RELATIVE_FORWARD {
		m.setPoint = m.curPosition + numTicks
	} else if d == pb.DirectionRelative_DIRECTION_RELATIVE_BACKWARD {
		m.setPoint = m.curPosition - numTicks
	} else {
		panic("impossible")
	}

	m.setRegulated(true)
	m.desiredRPM = rpm

	isOn, err := m.IsOn(ctx)
	if err != nil {
		return err
	}
	if !isOn {
		// if we're off we start slow, otherwise we just set the desired rpm
		err := m.doGo(ctx, d, .03)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *encodedMotor) Off(ctx context.Context) error {
	m.desiredRPM = 0.0
	m.setRegulated(false)
	return m.real.Off(ctx)
}

func (m *encodedMotor) IsOn(ctx context.Context) (bool, error) {
	return m.real.IsOn(ctx)
}
