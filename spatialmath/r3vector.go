package spatialmath

import (
	"math"

	"github.com/golang/geo/r3"
)

type r3VectorConfig struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// TranslationConfig represents the configuration format representing a translation between two objects - it is always in millimeters.
type TranslationConfig r3VectorConfig

// ParseConfig converts a TranslationConfig into a r3.Vector.
func (t TranslationConfig) ParseConfig() r3.Vector {
	return r3.Vector{t.X, t.Y, t.Z}
}

// AxisConfig represents the configuration format representing an axis.
type AxisConfig r3VectorConfig

// ParseConfig converts an AxisConfig into an R4AA object.
func (a AxisConfig) ParseConfig() R4AA {
	return R4AA{RX: a.X, RY: a.Y, RZ: a.Z}
}

// R3VectorAlmostEqual compares two r3.Vector objects and returns if the all elementwise differences are less than epsilon.
func R3VectorAlmostEqual(a, b r3.Vector, epsilon float64) bool {
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}
