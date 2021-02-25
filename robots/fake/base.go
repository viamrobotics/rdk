package fake

// tracks in CM
type Base struct {
}

func (b *Base) MoveStraight(distanceMM int, mmPerSec float64, block bool) error {
	return nil
}

func (b *Base) Spin(angleDeg float64, speed int, block bool) error {
	return nil
}

func (b *Base) Stop() error {
	return nil
}

func (b *Base) Close() {

}
