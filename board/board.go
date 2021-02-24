package board

import (
	"fmt"
)

// designed to match the gobot one for now
type Motor interface {
	Forward(speed byte) error
	Backward(speed byte) error
	Speed(speed byte) error
	Direction(d string) error // "forward", "backward", "none" // TODO(erh): not sure if i want to keep this
	Off() error
	IsOn() bool
}

type AnalogReader interface {
	Read() (int, error)
}

type DigitalInterrupt interface {
	// the number of times this interrupt has fired
	Count() int64
}

type Board interface {
	// nil if cannot find
	Motor(name string) Motor

	AnalogReader(name string) AnalogReader
	DigitalInterrupt(name string) DigitalInterrupt

	Close() error

	GetConfig() Config
}

func NewBoard(cfg Config) (Board, error) {
	switch cfg.Model {
	case "pi":
		return NewPiBoard(cfg)
	default:
		return nil, fmt.Errorf("unknown board model: %v", cfg.Model)
	}
}
