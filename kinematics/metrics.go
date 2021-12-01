package kinematics

import (
	"github.com/golang/geo/r3"

	spatial "go.viam.com/core/spatialmath"
)

// Metric defines a distance function to be minimized by gradient descent algorithms
type Metric func(spatial.Pose, spatial.Pose) float64

// NewSquaredNormMetric is the default distance function between two poses to be used for gradient descent
func NewSquaredNormMetric() Metric {
	return weightedSqNormDist
}

func weightedSqNormDist(from, to spatial.Pose) float64 {
	delta := spatial.PoseDelta(from, to)
	aa := delta.Orientation().AxisAngles()

	// Increase weight for orientation since it's a small number
	aaWeighted := (r3.Vector{aa.RX, aa.RY, aa.RZ}).Mul(10.0)

	return delta.Point().Norm2() + aaWeighted.Norm2()
}
