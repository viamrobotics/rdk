package kinematics

import (
	spatial "go.viam.com/core/spatialmath"
)

// Metric defines a distance function to be minimized by gradient descent algorithms
type Metric func(spatial.Pose, spatial.Pose) float64

// NewSquaredNormMetric is the default distance function between two poses to be used for gradient descent
func NewSquaredNormMetric() Metric {
	return weightedSqNormDist
}

func weightedSqNormDist(from, to spatial.Pose) float64 {
	d := spatial.PoseDelta(from, to)
	// Increase weight for orientation since it's a small number
	for i, v := range d {
		if i > 2 {
			d[i] = v * 10
		}
	}
	return SquaredNorm(d)
}
