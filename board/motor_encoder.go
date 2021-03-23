package board

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
)

var (
	rpmSleep = 50 * time.Millisecond // really just for testing
	rpmDebug = true
)

func WrapMotorWithEncoder(b Board, mc MotorConfig, m Motor, logger golog.Logger) (Motor, error) {
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
	mm2.rpmMonitorStart()

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

	lastForce    byte
	curDirection Direction
	setPoint     int64

	curPosition int64

	startedRPMMonitor bool
	rpmMonitorCalls   int64
	logger            golog.Logger
}

func (m *encodedMotor) Position() int64 {
	return m.curPosition
}

func (m *encodedMotor) PositionSupported() bool {
	return true
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

func (m *encodedMotor) Force(force byte) error {
	m.desiredRPM = 0 // if we're setting force manually, don't control RPM
	return m.setForce(force)
}

func (m *encodedMotor) setForce(force byte) error {
	m.lastForce = force
	return m.real.Force(force)
}

func (m *encodedMotor) Go(d Direction, force byte) error {
	m.setRegulated(false) // user wants direct control, so we stop trying to control the world
	m.desiredRPM = 0      // if we're setting force manually, don't control RPM
	return m.doGo(d, force)
}

func (m *encodedMotor) doGo(d Direction, force byte) error {
	m.lastForce = force
	m.curDirection = d
	return m.real.Go(d, force)
}

func (m *encodedMotor) rpmMonitorStart() {
	if m.startedRPMMonitor {
		return
	}
	go m.rpmMonitor()
}

func (m *encodedMotor) startRegulatorThread() {
	if m.encoderB == nil {
		m.startSingleEncoderThread()
	} else {
		m.startRotaryEncoderThread()
	}
}

func (m *encodedMotor) startSingleEncoderThread() {
	encoderChannel := make(chan bool)
	m.encoder.AddCallback(encoderChannel)
	go func() {
		for {
			stop := false

			<-encoderChannel

			if m.curDirection == DirForward {
				m.curPosition++
				stop = m.isRegulated() && m.curPosition >= m.setPoint
			} else if m.curDirection == DirBackward {
				m.curPosition--
				stop = m.isRegulated() && m.curPosition <= m.setPoint
			} else {
				m.logger.Warnf("got encoder tick but motor should be off")
			}

			if stop {
				err := m.Off()
				if err != nil {
					m.logger.Warnf("error turning motor off from after hit set point: %v", err)
				}
				m.setRegulated(false)
			}
		}
	}()
}

func (m *encodedMotor) startRotaryEncoderThread() {
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
				stop := (m.curDirection == DirForward && m.curPosition >= m.setPoint) ||
					(m.curDirection == DirBackward && m.curPosition <= m.setPoint)

				if stop {
					err := m.Off()
					if err != nil {
						m.logger.Warnf("error turning motor off from after hit set point: %v", err)
					}
					m.setRegulated(false)
				}
			}
		}
	}()

}

func (m *encodedMotor) rpmMonitor() {
	if m.encoder == nil {
		panic(fmt.Errorf("started rpmMonitor but have no encoder"))
	}

	if m.startedRPMMonitor {
		return
	}
	m.startedRPMMonitor = true

	// just a convenient place to start the encoder listener
	m.startRegulatorThread()

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

			var newForce byte

			if currentRPM == 0 {
				newForce = m.lastForce + 16
				if newForce < 16 {
					newForce = 255
				}
			} else {
				dOverC := m.desiredRPM / currentRPM
				if dOverC > 2 {
					dOverC = 2
				}
				neededForce := float64(m.lastForce) * dOverC

				if neededForce < 8 {
					neededForce = 8
				} else if neededForce > 255 {
					neededForce = 255
				}

				neededForce = (float64(m.lastForce) + neededForce) / 2 // slow down ramps

				newForce = byte(neededForce)
			}

			if newForce != m.lastForce {
				if rpmDebug {
					m.logger.Debugf("current rpm: %0.1f force: %v newForce: %v desiredRPM: %0.1f",
						currentRPM, m.lastForce, newForce, m.desiredRPM)
				}
				err := m.setForce(newForce)
				if err != nil {
					m.logger.Warnf("rpm regulator cannot set force %s", err)
				}
			}
		}

		lastCount = count
		lastTime = now
	}
}

func (m *encodedMotor) GoFor(d Direction, rpm float64, rotations float64) error {
	if d == DirNone {
		return m.Off()
	}

	if rotations < 0 {
		rotations *= -1
		d = FlipDirection(d)
	}

	if rotations == 0 {
		// users probably shouldn't do this, maybe we shouldn't support, but...
		return m.Go(d, 16) // force of 16 is random
	}

	m.rpmMonitorStart()

	numTicks := int64(rotations * float64(m.cfg.TicksPerRotation))

	if d == DirForward {
		m.setPoint = m.curPosition + numTicks
	} else if d == DirBackward {
		m.setPoint = m.curPosition - numTicks
	} else {
		panic("impossible")
	}

	m.setRegulated(true)
	m.desiredRPM = rpm

	if !m.IsOn() {
		// if we're off we start slow, otherwise we just set the desired rpm
		err := m.doGo(d, 8)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *encodedMotor) Off() error {
	m.desiredRPM = 0.0
	m.setRegulated(false)
	return m.real.Off()
}

func (m *encodedMotor) IsOn() bool {
	return m.real.IsOn()
}
