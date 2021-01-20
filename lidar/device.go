package lidar

import "image"

type Device interface {
	Start()
	Stop()
	Close()
	Scan() (Measurements, error)
	Range() int
	Bounds() (image.Point, error)
}
