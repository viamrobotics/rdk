package spatialmath

import (
	"github.com/golang/geo/r3"
)

func makeTestCapsule(o Orientation, pt r3.Vector, radius, length float64) Geometry {
	c, _ := NewCapsule(NewPose(pt, o), radius, length, "")
	return c
}
