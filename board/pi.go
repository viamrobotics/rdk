package board

import (
	"fmt"

	"go.uber.org/multierr"
	"gobot.io/x/gobot/drivers/aio"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/spi"
	"gobot.io/x/gobot/platforms/raspi"
	"gobot.io/x/gobot/sysfs"
)

func init() {
	RegisterBoard("pi", NewPiBoard)
}

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
	cfg   MotorConfig
	motor *gpio.MotorDriver
}

func (m *gobotMotor) Force(force byte) error {
	return m.setForce(force)
}

func (m *gobotMotor) setForce(force byte) error {
	return m.motor.Speed(force)
}

func (m *gobotMotor) Go(d Direction, force byte) error {
	dd := dirToGobot(d)
	return multierr.Combine(m.motor.Speed(force), m.motor.Direction(dd))
}

func (m *gobotMotor) GoFor(d Direction, rpm float64, rotations float64) error {
	return fmt.Errorf("gobotMotor doesn't support GoFor")
}

func (m *gobotMotor) Off() error {
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
	motors  []Motor
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

		var mm Motor = &gobotMotor{
			cfg:   mc,
			motor: m,
		}

		if mc.Encoder != "" {
			i := b.DigitalInterrupt(mc.Encoder)
			if i == nil {
				return nil, fmt.Errorf("cannot find encode (%s) for motor (%s)", mc.Encoder, mc.Name)
			}

			mm = &encodedMotor{
				cfg:     mc,
				real:    mm,
				encoder: i,
			}

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
