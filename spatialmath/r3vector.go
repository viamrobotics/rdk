package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

// R3VectorAlmostEqual compares two r3.Vector objects and returns if the all elementwise differences are less than epsilon.
func R3VectorAlmostEqual(a, b r3.Vector, epsilon float64) bool {
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}

// AxisConfig represents the configuration format representing an axis.
type AxisConfig r3.Vector

// NewAxisConfig constructs a config from an R4AA.
func NewAxisConfig(axis R4AA) *AxisConfig {
	return &AxisConfig{axis.RX, axis.RY, axis.RZ}
}

// ParseConfig converts an AxisConfig into an R4AA object.
func (a AxisConfig) ParseConfig() R4AA {
	return R4AA{RX: a.X, RY: a.Y, RZ: a.Z}
}
