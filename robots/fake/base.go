package fake

import (
	"errors"
)

// tracks in CM
type Base struct {
}

func (b *Base) MoveStraight(distanceMM int, speed int, block bool) error {
	return nil
}

func (b *Base) Spin(degrees int, power int, block bool) error {
	if degrees%90 != 0 {
		return errors.New("can only spin by 90 degree multiples")
	}
	return nil
}

func (b *Base) Stop() error {
	return nil
}

func (b *Base) Close() {

}
