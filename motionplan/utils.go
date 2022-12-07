package motionplan

import (
	"fmt"
	"math"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// FrameStepsFromRobotPath is a helper function which will extract the waypoints of a single frame from the map output of a robot path.
func FrameStepsFromRobotPath(frameName string, path []map[string][]referenceframe.Input) ([][]referenceframe.Input, error) {
	solution := make([][]referenceframe.Input, 0, len(path))
	for _, step := range path {
		frameStep, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in solved motion path", frameName)
		}
		solution = append(solution, frameStep)
	}
	return solution, nil
}

// PathStepCount will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func PathStepCount(seedPos, goalPos spatialmath.Pose, stepSize float64) int {
	// use a default size of 1 if zero is passed in to avoid divide-by-zero
	if stepSize == 0 {
		stepSize = 1.
	}

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatialmath.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(utils.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

// EvaluatePlan assigns a numeric score to a plan that corresponds to the cumulative distance between input waypoints in the plan.
func EvaluatePlan(plan [][]referenceframe.Input, distFunc Constraint) (totalCost float64) {
	if len(plan) < 2 {
		return math.Inf(1)
	}
	for i := 0; i < len(plan)-1; i++ {
		_, cost := distFunc(&ConstraintInput{StartInput: plan[i], EndInput: plan[i+1]})
		totalCost += cost
	}
	return totalCost
}

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
