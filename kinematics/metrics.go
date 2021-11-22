package kinematics

import (
	spatial "go.viam.com/core/spatialmath"
)

// Metric defines a distance function to be minimized by gradient descent algorithms
type Metric interface {
	Distance(spatial.Pose, spatial.Pose) float64
}

type flexibleMetric struct {
	f func(spatial.Pose, spatial.Pose) float64
}

// Distance returns the computed distance
func (m *flexibleMetric) Distance(from, to spatial.Pose) float64 {
	return m.f(from, to)
}

// NewBasicMetric wraps a function that then can be used as a Metric
func NewBasicMetric(f func(spatial.Pose, spatial.Pose) float64) Metric {
	return &flexibleMetric{f}
}

// NewSquaredNormMetric is the default distance function between two poses to be used for gradient descent
func NewSquaredNormMetric() Metric {
	return &flexibleMetric{sqNormDist}
}
func sqNormDist(from, to spatial.Pose) float64 {
	return SquaredNorm(spatial.PoseDelta(from, to))
}
