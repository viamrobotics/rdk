package board

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/edaniels/golog"
)

type encodedMotor struct {
	cfg     MotorConfig
	real    Motor
	encoder DigitalInterrupt

	regulated int32 // use atomic operations when access

	// TODO(erh): check thread safety on this
	desiredRPM float64 // <= 0 means thread should do nothing

	startedRPMMonitor bool
	lastForce         byte
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
	if m.isRegulated() {
		return fmt.Errorf("cannot control Force when motor in regulated mode")
	}
	m.desiredRPM = 0 // if we're setting force manually, don't control RPM
	return m.setForce(force)
}

func (m *encodedMotor) setForce(force byte) error {
	m.lastForce = force
	return m.real.Force(force)
}

func (m *encodedMotor) Go(d Direction, force byte) error {
	if m.isRegulated() {
		return fmt.Errorf("cannot tell motor to Go when motor in regulated mode")
	}
	m.desiredRPM = 0 // if we're setting force manually, don't control RPM
	m.lastForce = force
	//golog.Global.Debugf("encodedMotor d: %s MillisPerSec: %v", dd, speed)
	return m.real.Go(d, force)
}

func (m *encodedMotor) rpmMonitor() {
	if m.startedRPMMonitor {
		return
	}
	if m.encoder == nil {
		golog.Global.Warnf("started rpmMonitor but have no encode")
		return
	}

	m.startedRPMMonitor = true

	lastCount := m.encoder.Value()
	lastTime := time.Now().UnixNano()

	for {

		time.Sleep(50 * time.Millisecond)

		count := m.encoder.Value()
		now := time.Now().UnixNano()

		if m.desiredRPM > 0 {
			rotations := float64(count-lastCount) / float64(m.cfg.TicksPerRotation)
			minutes := float64(now-lastTime) / (1e9 * 60)
			currentRPM := rotations / minutes

			var newForce byte

			if currentRPM == 0 {
				newForce = m.lastForce + 16
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
				golog.Global.Debugf("current rpm: %0.1f force: %v newForce: %v desiredRPM: %0.1f",
					currentRPM, m.lastForce, newForce, m.desiredRPM)
				err := m.setForce(newForce)
				if err != nil {
					golog.Global.Warnf("rpm regulator cannot set force %s", err)
				}
			}
		}

		lastCount = count
		lastTime = now
	}
}

func (m *encodedMotor) GoFor(d Direction, millisPerSec float64, rotations float64, block bool) error {
	if m.isRegulated() {
		return fmt.Errorf("already running a GoFor directive, have to stop that first before can do another")
	}

	if rotations < 0 {
		return fmt.Errorf("rotations has to be >= 0")
	}

	//golog.Global.Debugf("m: %s d: %v MillisPerSec: %v rotations: %v block: %v", m.cfg.Name, d, speed, rotations, block)

	if m.encoder == nil {
		return fmt.Errorf("we don't have an encoder for motor %s", m.cfg.Name)
	}

	if !m.startedRPMMonitor {
		go m.rpmMonitor()
	}

	start := func() error {
		if !m.IsOn() {
			// if we're off we start slow, otherwise we just set the desired rpm
			err := m.Go(d, 8)
			if err != nil {
				return err
			}
		}
		m.desiredRPM = millisPerSec
		return nil
	}

	if rotations == 0 {
		// go forever
		if block {
			return fmt.Errorf("you cannot block if you don't set a number of rotations")
		}

		return start()
	}

	numTicks := rotations * float64(m.cfg.TicksPerRotation)

	done := make(chan int64)
	m.encoder.AddCallbackDelta(int64(numTicks), done)

	err := start()
	if err != nil {
		return err
	}

	m.setRegulated(true)

	finish := func() error {
		<-done
		m.setRegulated(false)
		return m.Off()
	}

	if !block {
		go func() {
			err := finish()
			if err != nil {
				golog.Global.Warnf("after non-blocking move, could not stop motor (%s): %s", m.cfg.Name, err)
			}
		}()
		return nil
	}

	return finish()
}

func (m *encodedMotor) Off() error {
	m.desiredRPM = 0.0
	if m.isRegulated() {
		golog.Global.Warnf("turning motor off while in regulated mode, this could break things")
	}
	return m.real.Off()
}

func (m *encodedMotor) IsOn() bool {
	return m.real.IsOn()
}
