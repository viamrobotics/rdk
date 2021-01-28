package base

type Base interface {
	MoveStraight(distanceMM int, speed int, block bool) error
	Spin(degrees int, power int, block bool) error
	Stop() error
	Close()
}
