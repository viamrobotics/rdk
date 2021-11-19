package kinematics

import (
	spatial "go.viam.com/core/spatialmath"
)

type Metric interface{
	Distance(spatial.Pose, spatial.Pose) float64
}

type flexibleMetric struct{
	f func(spatial.Pose, spatial.Pose) float64
}

func (m *flexibleMetric) Distance(from, to spatial.Pose) float64 {
	return m.f(from, to)
}

func NewBasicMetric(f func(spatial.Pose, spatial.Pose) float64) Metric {
	return &flexibleMetric{f}
}

// SquaredNormMetric is the default distance function between two poses to be used for gradient descent
func NewSquaredNormMetric() Metric {
	return &flexibleMetric{sqNormDist}
}
func sqNormDist(from, to spatial.Pose) float64 {
	return SquaredNorm(spatial.PoseDelta(from, to))
}
