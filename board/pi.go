package board

import (
	"fmt"

	"gobot.io/x/gobot/drivers/aio"
	"gobot.io/x/gobot/drivers/gpio"
	"gobot.io/x/gobot/drivers/spi"
	"gobot.io/x/gobot/platforms/raspi"
)

type gobotAnalogReader struct {
	r   aio.AnalogReader
	pin string
}

func (r *gobotAnalogReader) Read() (int, error) {
	return r.r.AnalogRead(r.pin)
}

type piBoard struct {
	cfg    Config
	r      *raspi.Adaptor
	motors []*gpio.MotorDriver
	ar     aio.AnalogReader
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
	return pi.r.Finalize()
}

func NewPiBoard(cfg Config) (Board, error) {
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
		err := mcp.Start()
		if err != nil {
			return nil, err
		}
		b.ar = mcp
	}

	return b, nil
}
