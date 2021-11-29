package kinematics

import (
	spatial "go.viam.com/core/spatialmath"
)

// Metric defines a distance function to be minimized by gradient descent algorithms
type Metric func(spatial.Pose, spatial.Pose) float64

// NewSquaredNormMetric is the default distance function between two poses to be used for gradient descent
func NewSquaredNormMetric() Metric {
	return sqNormDist
}
func sqNormDist(from, to spatial.Pose) float64 {
	return SquaredNorm(spatial.PoseDelta(from, to))
}
