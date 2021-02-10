package fake

// tracks in CM
type Base struct {
}

func (b *Base) MoveStraight(distanceMM int, speed int, block bool) error {
	return nil
}

func (b *Base) Spin(degrees float64, power int, block bool) error {
	return nil
}

func (b *Base) Stop() error {
	return nil
}

func (b *Base) Close() {

}
