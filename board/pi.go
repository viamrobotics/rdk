package board

import (
	"fmt"

	"gobot.io/x/gobot/drivers/aio"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/spi"
	"gobot.io/x/gobot/platforms/raspi"
	"gobot.io/x/gobot/sysfs"

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

type sysfsDigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	count int64
}

func (i *sysfsDigitalInterrupt) Count() int64 {
	return i.count
}

// ----

type piBoard struct {
	cfg    Config
	r      *raspi.Adaptor
	motors []*gpio.MotorDriver
	ar     aio.AnalogReader

	sysfsListner *sysfs.InterruptListener
	interrupts   []*sysfsDigitalInterrupt
}

func (pi *piBoard) GetConfig() Config {
	return pi.cfg
}

func (pi *piBoard) DigitalInterrupt(name string) DigitalInterrupt {
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

	for _, mc := range cfg.Motors {
		for _, s := range []string{"a", "b", "pwm"} {
			if mc.Pins[s] == "" {
				return nil, fmt.Errorf("motor [%s] missing pin: %s", mc.Name, s)
			}
		}
		m := gpio.NewMotorDriver(b.r, mc.Pins["pwm"])
		m.ForwardPin = mc.Pins["a"]
		m.BackwardPin = mc.Pins["b"]
		b.motors = append(b.motors, m)
	}

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
		b.sysfsListner.Start()

		for _, di := range cfg.DigitalInterrupts {
			t := &sysfsDigitalInterrupt{di, 0}
			b.interrupts = append(b.interrupts, t)

			pin, err := b.r.DigitalPin(di.Pin, "")
			if err != nil {
				return nil, err
			}

			err = pin.Listen(di.Mode, b.sysfsListner, func(b byte) {
				t.count++
			})
		}

	}
	return b, nil
}
