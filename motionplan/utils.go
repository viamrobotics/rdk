package motionplan

import (
	"math"

	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// CalculateStepCount will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
// stepSize represents both the max mm movement per step, and max R4AA degrees per step.
func CalculateStepCount(seedPos, goalPos spatialmath.Pose, stepSize float64) int {
	// use a default size of 1 if zero is passed in to avoid divide-by-zero
	if stepSize == 0 {
		stepSize = 1.
	}

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatialmath.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/stepSize), math.Abs(utils.RadToDeg(rDist.Theta)/stepSize))
	return int(nSteps) + 1
}

func calculateJointStepCount(start, end []float64, stepSize float64) int {
	steps := 0
	for idx, s := range start {
		if idx >= len(end) {
			break
		}
		mySteps := int(math.Ceil(math.Abs(end[idx]-s) / stepSize))
		steps = max(mySteps, steps)
	}
	return steps
}
