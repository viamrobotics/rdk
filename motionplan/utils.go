package motionplan

import (
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// fixOvIncrement will detect whether the given goal position is a precise orientation increment of the current
// position, in which case it will detect whether we are leaving a pole. If we are an OV increment and leaving a pole,
// then Theta will be adjusted to give an expected smooth movement. The adjusted goal will be returned. Otherwise the
// original goal is returned.
// Rationale: if clicking the increment buttons in the interface, the user likely wants the most intuitive motion
// posible. If setting values manually, the user likely wants exactly what they requested.
func fixOvIncrement(goal, seed spatialmath.Pose) spatialmath.Pose {
	epsilon := 0.01
	goalPt := goal.Point()
	goalOrientation := goal.Orientation().OrientationVectorDegrees()
	seedPt := seed.Point()
	seedOrientation := seed.Orientation().OrientationVectorDegrees()

	// Nothing to do for spatial translations or theta increments
	r := utils.Float64AlmostEqual(goalOrientation.OZ, seedOrientation.OZ, epsilon)
	_ = r
	if !spatialmath.R3VectorAlmostEqual(goalPt, seedPt, epsilon) ||
		!utils.Float64AlmostEqual(goalOrientation.Theta, seedOrientation.Theta, epsilon) {
		return goal
	}
	// Check if seed is pointing directly at pole
	if 1-math.Abs(seedOrientation.OZ) > epsilon || !utils.Float64AlmostEqual(goalOrientation.OZ, seedOrientation.OZ, epsilon) {
		return goal
	}

	// we only care about negative xInc
	xInc := goalOrientation.OX - seedOrientation.OX
	yInc := math.Abs(goalOrientation.OY - seedOrientation.OY)
	var adj float64
	if utils.Float64AlmostEqual(goalOrientation.OX, seedOrientation.OX, epsilon) {
		// no OX movement
		if !utils.Float64AlmostEqual(yInc, 0.1, epsilon) && !utils.Float64AlmostEqual(yInc, 0.01, epsilon) {
			// nonstandard increment
			return goal
		}
		// If wanting to point towards +Y and OZ<0, add 90 to theta, otherwise subtract 90
		if goalOrientation.OY-seedOrientation.OY > 0 {
			adj = 90
		} else {
			adj = -90
		}
	} else {
		if (!utils.Float64AlmostEqual(xInc, -0.1, epsilon) && !utils.Float64AlmostEqual(xInc, -0.01, epsilon)) ||
			!utils.Float64AlmostEqual(goalOrientation.OY, seedOrientation.OY, epsilon) {
			return goal
		}
		// If wanting to point towards -X, increment by 180. Values over 180 or under -180 will be automatically wrapped
		adj = 180
	}
	if goalOrientation.OZ > 0 {
		adj *= -1
	}
	goalOrientation.Theta += adj

	return spatialmath.NewPoseFromOrientation(goalPt, goalOrientation)
}

func stepsToNodes(steps [][]referenceframe.Input) []node {
	nodes := make([]node, 0, len(steps))
	for _, step := range steps {
		nodes = append(nodes, &basicNode{step})
	}
	return nodes
}
