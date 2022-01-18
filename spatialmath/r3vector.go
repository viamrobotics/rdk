package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

type R3VectorConfig struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (config R3VectorConfig) Unmarshal() r3.Vector {
	return r3.Vector{config.X, config.Y, config.Z}
}

// Translation is an alias for R3VectorConfig and representsthe translation between two objects in the grid system.
// It is always in millimeters.
type Translation R3VectorConfig

func (t Translation) Unmarshal() r3.Vector {
	return r3.Vector{t.X, t.Y, t.Z}
}

type Axis R3VectorConfig

func (a Axis) Unmarshal() R4AA {
	return R4AA{RX: a.X, RY: a.Y, RZ: a.Z}
}

// R3VectorAlmostEqual compares two r3.Vector objects and returns if the all elementwise differences are less than epsilon.
func R3VectorAlmostEqual(a, b r3.Vector, epsilon float64) bool {
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}
