package lrf

type LaserRangeFinder interface {
	Start()
	Stop()
	Scan() (Measurements, error)
	Range() int
}
