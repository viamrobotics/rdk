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

// designed to match the gobot one for now
type Motor interface {
	Speed(speed byte) error
	// "forward", "backward", "none" // TODO(erh): not sure if i want to keep this
	Go(d Direction, speed byte) error

	GoFor(d Direction, speed byte, rotations float64, block bool) error

	Off() error
	IsOn() bool
}

type AnalogReader interface {
	Read() (int, error)
}

type Board interface {
	// nil if cannot find
	Motor(name string) Motor

	AnalogReader(name string) AnalogReader
	DigitalInterrupt(name string) *DigitalInterrupt

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

// should this be an interface
type DigitalInterrupt struct {
	cfg   DigitalInterruptConfig
	count int64

	callbacks []diCallback
}

func (i *DigitalInterrupt) Count() int64 {
	return i.count
}

func (i *DigitalInterrupt) tick() {
	i.count++

	for {
		got := false

		for idx, c := range i.callbacks {
			if i.count < c.threshold {
				continue
			}

			c.c <- i.count
			i.callbacks = append(i.callbacks[0:idx], i.callbacks[idx+1:]...)
			got = true
			break
		}
		if !got {
			break
		}
	}
}

func (i *DigitalInterrupt) AddCallbackDelta(delta int64, c chan int64) {
	i.callbacks = append(i.callbacks, diCallback{i.count + delta, c})
}
