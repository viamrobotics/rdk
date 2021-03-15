package board

import (
	"fmt"
	"sync/atomic"
	"time"

	"go.uber.org/multierr"
	"gobot.io/x/gobot/drivers/aio"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/spi"
	"gobot.io/x/gobot/platforms/raspi"
	"gobot.io/x/gobot/sysfs"

	"github.com/edaniels/golog"
)

type gobotAnalogReader struct {
	r   aio.AnalogReader
	pin string
}

func (r *gobotAnalogReader) Read() (int, error) {
	return r.r.AnalogRead(r.pin)
}

// ----

func dirToGobot(d Direction) string {
	switch d {
	case DirNone:
		return "none"
	case DirForward:
		return "forward"
	case DirBackward:
		return "backward"
	default:
		panic(fmt.Errorf("unknown direction %v", d))
	}
}

type gobotMotor struct {
	cfg     MotorConfig
	motor   *gpio.MotorDriver
	encoder DigitalInterrupt

	regulated int32 // use atomic operations when access

	// TODO(erh): check thread safety on this
	desiredRPM float64 // <= 0 means thread should do nothing

	startedRPMMonitor bool
	lastForce         byte
}

func (m *gobotMotor) isRegulated() bool {
	return atomic.LoadInt32(&m.regulated) == 1
}

func (m *gobotMotor) setRegulated(b bool) {
	if b {
		atomic.StoreInt32(&m.regulated, 1)
	} else {
		atomic.StoreInt32(&m.regulated, 0)
	}
}

func (m *gobotMotor) Force(force byte) error {
	if m.isRegulated() {
		return fmt.Errorf("cannot control Force when motor in regulated mode")
	}
	m.desiredRPM = 0 // if we're setting force manually, don't control RPM
	return m.setForce(force)
}

func (m *gobotMotor) setForce(force byte) error {
	m.lastForce = force
	return m.motor.Speed(force)
}

func (m *gobotMotor) Go(d Direction, force byte) error {
	if m.isRegulated() {
		return fmt.Errorf("cannot tell motor to Go when motor in regulated mode")
	}
	m.desiredRPM = 0 // if we're setting force manually, don't control RPM
	m.lastForce = force
	dd := dirToGobot(d)
	//golog.Global.Debugf("gobotMotor d: %s MillisPerSec: %v", dd, speed)
	return multierr.Combine(m.motor.Speed(force), m.motor.Direction(dd))
}

func (m *gobotMotor) rpmMonitor() {
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

func (m *gobotMotor) GoFor(d Direction, millisPerSec float64, rotations float64, block bool) error {
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

func (m *gobotMotor) Off() error {
	m.desiredRPM = 0.0
	if m.isRegulated() {
		golog.Global.Warnf("turning motor off while in regulated mode, this could break things")
	}
	return m.motor.Off()
}

func (m *gobotMotor) IsOn() bool {
	return m.motor.IsOn()
}

// ----

type gobotServo struct {
	driver  *gpio.ServoDriver
	current uint8
}

func (s *gobotServo) Move(angle uint8) error {
	s.current = angle
	return s.driver.Move(angle)
}
func (s *gobotServo) Current() uint8 {
	return s.current
}

type piBoard struct {
	cfg     Config
	r       *raspi.Adaptor
	motors  []*gobotMotor
	ar      aio.AnalogReader
	servos  []*gobotServo
	analogs map[string]AnalogReader

	sysfsListner *sysfs.InterruptListener
	interrupts   []DigitalInterrupt
}

func (pi *piBoard) GetConfig() Config {
	return pi.cfg
}

func (pi *piBoard) DigitalInterrupt(name string) DigitalInterrupt {
	for _, i := range pi.interrupts {
		if i.Config().Name == name {
			return i

		}
	}

	return nil
}

func (pi *piBoard) Motor(name string) Motor {
	for idx, mc := range pi.cfg.Motors {
		if name == mc.Name {
			return pi.motors[idx]
		}
	}
	return nil
}

func (pi *piBoard) Servo(name string) Servo {
	for idx, sc := range pi.cfg.Servos {
		if name == sc.Name {
			return pi.servos[idx]
		}
	}
	return nil
}

func (pi *piBoard) AnalogReader(name string) AnalogReader {
	a, ok := pi.analogs[name]
	if ok && a != nil {
		return a
	}

	for _, ac := range pi.cfg.Analogs {
		if name == ac.Name {
			a = &gobotAnalogReader{pi.ar, ac.Pin}
			if ac.AverageOverMillis > 0 {
				as := &AnalogSmoother{
					Raw:               a,
					AverageOverMillis: ac.AverageOverMillis,
					SamplesPerSecond:  ac.SamplesPerSecond,
				}
				as.Start()
				a = as
			}

			if pi.analogs == nil {
				pi.analogs = map[string]AnalogReader{}
			}

			pi.analogs[name] = a
			return a
		}
	}

	return nil
}

func (pi *piBoard) Close() error {
	err := []error{}

	if pi.sysfsListner != nil {
		err = append(err, pi.sysfsListner.Close())
	}
	err = append(err, pi.r.Finalize())

	return multierr.Combine(err...)
}

func NewPiBoard(cfg Config) (Board, error) {
	var err error

	b := &piBoard{}
	b.cfg = cfg
	b.r = raspi.NewAdaptor()

	if len(cfg.Analogs) > 0 {
		mcp := spi.NewMCP3008Driver(b.r)
		err = mcp.Start()
		if err != nil {
			return nil, err
		}
		b.ar = mcp
	}

	if len(cfg.DigitalInterrupts) > 0 {
		b.sysfsListner, err = sysfs.NewInterruptListener()
		if err != nil {
			return nil, err
		}
		err = b.sysfsListner.Start()
		if err != nil {
			return nil, err
		}

		for _, di := range cfg.DigitalInterrupts {
			t, err := createDigitalInterrupt(di)
			if err != nil {
				return nil, err
			}
			b.interrupts = append(b.interrupts, t)

			err = b.r.DigitalPinSetPullUpDown(di.Pin, true) // TODO(erh): make this configurable, but for most things we want up
			if err != nil {
				return nil, err
			}

			pin, err := b.r.DigitalPin(di.Pin, "")
			if err != nil {
				return nil, err
			}

			err = pin.Listen(di.Mode, b.sysfsListner, func(b byte) {
				t.Tick()
			})
			if err != nil {
				return nil, err
			}
		}

	}

	for _, mc := range cfg.Motors {
		for _, s := range []string{"a", "b", "pwm"} {
			if mc.Pins[s] == "" {
				return nil, fmt.Errorf("motor [%s] missing pin: %s", mc.Name, s)
			}
		}
		m := gpio.NewMotorDriver(b.r, mc.Pins["pwm"])
		m.ForwardPin = mc.Pins["a"]
		m.BackwardPin = mc.Pins["b"]

		mm := &gobotMotor{}
		mm.cfg = mc
		mm.motor = m

		if mc.Encoder != "" {
			i := b.DigitalInterrupt(mc.Encoder)
			if i == nil {
				return nil, fmt.Errorf("cannot find encode (%s) for motor (%s)", mc.Encoder, mc.Name)
			}
			mm.encoder = i
		}

		b.motors = append(b.motors, mm)
	}

	for _, sc := range cfg.Servos {
		_, err := b.r.DigitalPin(sc.Pin, "out")
		if err != nil {
			return nil, err
		}
		b.servos = append(b.servos, &gobotServo{gpio.NewServoDriver(b.r, sc.Pin), 0})
	}

	return b, nil
}
