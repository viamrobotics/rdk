package board

import (
	"fmt"
)

type MotorStatus struct {
	On                bool
	PositionSupported bool
	Position          int64
}

type ServoStatus struct {
	Angle uint8
}

type AnalogStatus struct {
	Value int
}

type DigitalInterruptStatus struct {
	Value int64
}

type Status struct {
	Motors            map[string]MotorStatus
	Servos            map[string]ServoStatus
	Analogs           map[string]AnalogStatus
	DigitalInterrupts map[string]DigitalInterruptStatus
}

func CreateStatus(b Board) (Status, error) {
	s := Status{
		Motors:            map[string]MotorStatus{},
		Servos:            map[string]ServoStatus{},
		Analogs:           map[string]AnalogStatus{},
		DigitalInterrupts: map[string]DigitalInterruptStatus{},
	}

	cfg := b.GetConfig()

	for _, c := range cfg.Motors {
		name := c.Name
		x := b.Motor(name)
		s.Motors[name] = MotorStatus{
			On:                x.IsOn(),
			Position:          x.Position(),
			PositionSupported: x.PositionSupported(),
		}
	}

	for _, c := range cfg.Servos {
		name := c.Name
		x := b.Servo(name)
		s.Servos[name] = ServoStatus{x.Current()}
	}

	for _, c := range cfg.Analogs {
		name := c.Name
		x := b.AnalogReader(name)
		val, err := x.Read()
		if err != nil {
			return s, fmt.Errorf("couldn't read analog (%s) : %s", name, err)
		}
		s.Analogs[name] = AnalogStatus{val}
	}

	for _, c := range cfg.DigitalInterrupts {
		name := c.Name
		x := b.DigitalInterrupt(name)
		s.DigitalInterrupts[name] = DigitalInterruptStatus{x.Value()}
	}

	return s, nil
}
