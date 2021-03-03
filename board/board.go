package board

import (
	"fmt"
)

type Direction int

const (
	DirNone     = 0
	DirForward  = 1
	DirBackward = 2
)

func DirectionFromString(s string) Direction {
	if len(s) == 0 {
		return DirNone
	}

	if s[0] == 'f' {
		return DirForward
	}

	if s[0] == 'b' {
		return DirBackward
	}

	return DirNone
}

type Motor interface {
	Force(force byte) error

	Go(d Direction, force byte) error

	GoFor(d Direction, speed float64, rotations float64, block bool) error

	Off() error
	IsOn() bool
}

type Servo interface {
	// moves to that angle (0-180)
	Move(angle uint8) error
	Current() uint8
}

type AnalogReader interface {
	Read() (int, error)
}

type Board interface {
	// nil if cannot find
	Motor(name string) Motor
	Servo(name string) Servo

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

type diCallback struct {
	threshold int64
	c         chan int64
}

type DigitalInterrupt interface {
	Config() DigitalInterruptConfig
	Value() int64
	Tick()
	AddCallbackDelta(delta int64, c chan int64)
}
