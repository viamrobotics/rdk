package support

import (
	"errors"
)

// tracks in CM
type FakeBase struct {
	orientation int
}

func (fb *FakeBase) MoveStraight(distanceMM int, speed int, block bool) error {
	return nil
}

func (fb *FakeBase) Spin(degrees int, power int, block bool) error {
	if degrees%90 != 0 {
		return errors.New("can only spin by 90 degree multiples")
	}
	fb.orientation = (((fb.orientation + degrees) % 360) + 360) % 360
	return nil
}

func (fb *FakeBase) Orientation() int {
	return fb.orientation
}

func (fb *FakeBase) Stop() error {
	return nil
}

func (fb *FakeBase) Close() {

}
