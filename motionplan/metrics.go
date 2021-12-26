package motionplan

import (
	spatial "go.viam.com/rdk/spatialmath"
)

// Metric defines a distance function to be minimized by gradient descent algorithms.
type Metric func(spatial.Pose, spatial.Pose) float64

// NewSquaredNormMetric is the default distance function between two poses to be used for gradient descent.
func NewSquaredNormMetric() Metric {
	return weightedSqNormDist
}

func weightedSqNormDist(from, to spatial.Pose) float64 {
	delta := spatial.PoseDelta(from, to)
	// Increase weight for orientation since it's a small number
	return delta.Point().Norm2() + spatial.QuatToR3AA(delta.Orientation().Quaternion()).Mul(10.).Norm2()
}
