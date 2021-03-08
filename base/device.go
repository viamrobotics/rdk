package base

type Device interface {
	MoveStraight(distanceMM int, mmPerSec float64, block bool) error
	Spin(angleDeg float64, speed int, block bool) error
	Stop() error
	Close()
	Width() float64
}
