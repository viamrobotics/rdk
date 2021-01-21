package base

import (
	"fmt"
)

type Action struct {
	DistanceMM int
	Angle      int

	Speed int
	Block bool
}

func (a Action) Move(b Base) error {
	if a.DistanceMM != 0 && a.Angle != 0 {
		return fmt.Errorf("can only specify DistanceMM or Angle for now")
	}

	if a.DistanceMM != 0 {
		return b.MoveStraight(a.DistanceMM, a.Speed, a.Block)
	}

	if a.Angle != 0 {
		return b.Spin(a.Angle, a.Speed, a.Block)
	}

	return fmt.Errorf("need to specify DistanceMM or Angle")
}

type Base interface {
	MoveStraight(distanceMM int, speed int, block bool) error
	Spin(degrees int, power int, block bool) error
	Stop() error
	Close()
}
