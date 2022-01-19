package motionplan

import (
	"math"

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

type combinableMetric struct {
	metrics []Metric
}

func (m *combinableMetric) combinedDist(p1, p2 spatial.Pose) float64 {
	dist := 0.
	for _, metric := range m.metrics {
		dist = +metric(p1, p2)
	}
	return dist
}

// CombineMetrics will take a variable number of Metrics and return a new Metric which will combine all given metrics into one, summing
// their distances.
func CombineMetrics(metrics ...Metric) Metric {
	cm := &combinableMetric{metrics: metrics}
	return cm.combinedDist
}

func newDefaultMetric(start, end spatial.Pose) Metric {
	delta := spatial.PoseDelta(start, end)
	tDist := delta.Point().Norm2() * deviationFactor
	oDist := spatial.QuatToR3AA(delta.Orientation().Quaternion()).Norm2() * deviationFactor
	endpoints := []spatial.Pose{start, end}
	return func(from, to spatial.Pose) float64 {
		minDist := math.Inf(1)
		for _, endpoint := range endpoints {
			dist := 0.
			delta := spatial.PoseDelta(from, endpoint)
			transDist := delta.Point().Norm2()
			orientDist := spatial.QuatToR3AA(delta.Orientation().Quaternion()).Norm2()
			if transDist > tDist {
				dist += transDist - tDist
			}
			if orientDist > oDist {
				dist = +orientDist - oDist
			}
			if dist == 0. {
				return dist
			}
			if dist < minDist {
				minDist = dist
			}
		}
		return minDist
	}
}
