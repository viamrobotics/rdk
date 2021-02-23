package base

type Device interface {
	MoveStraight(distanceMM int, speed int, block bool) error
	Spin(angleDeg float64, speed int, block bool) error
	Stop() error
	Close()
}
