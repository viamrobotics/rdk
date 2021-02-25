package board

import (
	"fmt"
	"sync/atomic"

	"gobot.io/x/gobot/drivers/aio"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/spi"
	"gobot.io/x/gobot/platforms/raspi"
	"gobot.io/x/gobot/sysfs"

	"github.com/edaniels/golog"

	"github.com/viamrobotics/robotcore/utils"
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
	encoder *DigitalInterrupt

	regulated int32 // use atomic operations when access
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
	return m.motor.Speed(force)
}

func (m *gobotMotor) Go(d Direction, speed byte) error {
	if m.isRegulated() {
		return fmt.Errorf("cannot tell motor to Go when motor in regulated mode")
	}
	dd := dirToGobot(d)
	//golog.Global.Debugf("gobotMotor d: %s speed: %v", dd, speed)
	return utils.CombineErrors(m.motor.Speed(speed), m.motor.Direction(dd))
}

func (m *gobotMotor) GoFor(d Direction, speed byte, rotations float64, block bool) error {
	if m.isRegulated() {
		return fmt.Errorf("already running a GoFor directive, have to stop that first before can do another")
	}

	if rotations < 0 {
		return fmt.Errorf("rotations has to be >= 0")
	}

	golog.Global.Debugf("m: %s d: %v speed: %v rotations: %v block: %v", m.cfg.Name, d, speed, rotations, block)

	if m.encoder == nil {
		return fmt.Errorf("we don't have an encoder for motor %s", m.cfg.Name)
	}

	if rotations == 0 {
		// go forever
		if block {
			return fmt.Errorf("you cannot block if you don't set a number of rotations")
		}

		return m.Go(d, speed)
	}

	numTicks := rotations * float64(m.cfg.TicksPerRotation)

	done := make(chan int64)
	m.encoder.AddCallbackDelta(int64(numTicks), done)

	err := m.Go(d, speed)
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
	if m.isRegulated() {
		golog.Global.Warnf("turning motor off while in regulated mode, this could break things")
	}
	return m.motor.Off()
}

func (m *gobotMotor) IsOn() bool {
	return m.motor.IsOn()
}

// ----

type piBoard struct {
	cfg    Config
	r      *raspi.Adaptor
	motors []*gobotMotor
	ar     aio.AnalogReader

	sysfsListner *sysfs.InterruptListener
	interrupts   []*DigitalInterrupt
}

func (pi *piBoard) GetConfig() Config {
	return pi.cfg
}

func (pi *piBoard) DigitalInterrupt(name string) *DigitalInterrupt {
	for _, i := range pi.interrupts {
		if i.cfg.Name == name {
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

func (pi *piBoard) AnalogReader(name string) AnalogReader {
	for _, ac := range pi.cfg.Analogs {
		if name == ac.Name {
			return &gobotAnalogReader{pi.ar, ac.Pin}
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

	return utils.CombineErrors(err...)
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
			t := &DigitalInterrupt{cfg: di, count: 0}
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
				t.tick()
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

	return b, nil
}
