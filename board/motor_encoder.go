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

	lastPower    byte
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

func (m *encodedMotor) Power(ctx context.Context, power byte) error {
	m.desiredRPM = 0 // if we're setting power manually, don't control RPM
	return m.setPower(ctx, power)
}

func (m *encodedMotor) setPower(ctx context.Context, power byte) error {
	m.lastPower = power
	return m.real.Power(ctx, power)
}

func (m *encodedMotor) Go(ctx context.Context, d pb.DirectionRelative, power byte) error {
	m.setRegulated(false) // user wants direct control, so we stop trying to control the world
	m.desiredRPM = 0      // if we're setting power manually, don't control RPM
	return m.doGo(ctx, d, power)
}

func (m *encodedMotor) doGo(ctx context.Context, d pb.DirectionRelative, power byte) error {
	m.lastPower = power
	m.curDirection = d
	return m.real.Go(ctx, d, power)
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
			} else {
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

			var newPower byte

			if currentRPM == 0 {
				newPower = m.lastPower + 16
				if newPower < 16 {
					newPower = 255
				}
			} else {
				dOverC := m.desiredRPM / currentRPM
				if dOverC > 2 {
					dOverC = 2
				}
				neededPower := float64(m.lastPower) * dOverC

				if neededPower < 8 {
					neededPower = 8
				} else if neededPower > 255 {
					neededPower = 255
				}

				neededPower = (float64(m.lastPower) + neededPower) / 2 // slow down ramps

				newPower = byte(neededPower)
			}

			if newPower != m.lastPower {
				if rpmDebug {
					m.logger.Debugf("current rpm: %0.1f power: %v newPower: %v desiredRPM: %0.1f",
						currentRPM, m.lastPower, newPower, m.desiredRPM)
				}
				err := m.setPower(ctx, newPower)
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
		return m.doGo(ctx, d, 16) // power of 16 is random
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
		err := m.doGo(ctx, d, 8)
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
