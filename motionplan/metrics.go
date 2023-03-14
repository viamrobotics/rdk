package motionplan

import (
	"math"

	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
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

// NewZeroMetric always returns zero as the distance between two points.
func NewZeroMetric() Metric {
	return zeroDist
}

func zeroDist(from, to spatial.Pose) float64 {
	return 0
}

type combinableMetric struct {
	metrics []Metric
}

func (m *combinableMetric) combinedDist(p1, p2 spatial.Pose) float64 {
	dist := 0.
	for _, metric := range m.metrics {
		dist += metric(p1, p2)
	}
	return dist
}

// CombineMetrics will take a variable number of Metrics and return a new Metric which will combine all given metrics into one, summing
// their distances.
func CombineMetrics(metrics ...Metric) Metric {
	cm := &combinableMetric{metrics: metrics}
	return cm.combinedDist
}

// orientDist returns the arclength between two orientations in degrees.
func orientDist(o1, o2 spatial.Orientation) float64 {
	return utils.RadToDeg(spatial.QuatToR4AA(spatial.OrientationBetween(o1, o2).Quaternion()).Theta)
}

// orientDistToRegion will return a function which will tell you how far the unit sphere component of an orientation
// vector is from a region defined by a point and an arclength around it. The theta value of OV is disregarded.
// This is useful, for example, in defining the set of acceptable angles of attack for writing on a whiteboard.
func orientDistToRegion(goal spatial.Orientation, alpha float64) func(spatial.Orientation) float64 {
	ov1 := goal.OrientationVectorRadians()
	return func(o spatial.Orientation) float64 {
		ov2 := o.OrientationVectorRadians()
		acosInput := ov1.OX*ov2.OX + ov1.OY*ov2.OY + ov1.OZ*ov2.OZ

		// Account for floating point issues
		if acosInput > 1.0 {
			acosInput = 1.0
		}
		if acosInput < -1.0 {
			acosInput = -1.0
		}
		dist := math.Acos(acosInput)
		return math.Max(0, dist-alpha)
	}
}

// NewPoseFlexOVMetric will provide a distance function which will converge on an OV within an arclength of `alpha`
// of the ov of the goal given. The 3d point of the goal given is discarded, and the function will converge on the
// 3d point of the `to` pose (this is probably what you want).
func NewPoseFlexOVMetric(goal spatial.Pose, alpha float64) Metric {
	oDistFunc := orientDistToRegion(goal.Orientation(), alpha)
	return func(from, to spatial.Pose) float64 {
		pDist := from.Point().Distance(to.Point())
		oDist := oDistFunc(from.Orientation())
		return pDist*pDist + oDist*oDist
	}
}
