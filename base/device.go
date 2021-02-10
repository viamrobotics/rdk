package base

type Device interface {
	MoveStraight(distanceMM int, speed int, block bool) error
	Spin(degrees float64, power int, block bool) error
	Stop() error
	Close()
}
