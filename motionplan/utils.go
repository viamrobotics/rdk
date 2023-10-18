//go:build !no_cgo

package motionplan

import (
	"context"
	"fmt"
	"math"

	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// Plan describes a motion plan.
type Plan []map[string][]referenceframe.Input

// GetFrameSteps is a helper function which will extract the waypoints of a single frame from the map output of a robot path.
func (plan Plan) GetFrameSteps(frameName string) ([][]referenceframe.Input, error) {
	solution := make([][]referenceframe.Input, 0, len(plan))
	for _, step := range plan {
		frameStep, ok := step[frameName]
		if !ok {
			return nil, fmt.Errorf("frame named %s not found in solved motion plan", frameName)
		}
		solution = append(solution, frameStep)
	}
	return solution, nil
}

// String returns a human-readable version of the Plan, suitable for debugging.
func (plan Plan) String() string {
	var str string
	for _, step := range plan {
		str += "\n"
		for component, input := range step {
			if len(input) > 0 {
				str += fmt.Sprintf("%s: %v\t", component, input)
			}
		}
	}
	return str
}

// Evaluate assigns a numeric score to a plan that corresponds to the cumulative distance between input waypoints in the plan.
func (plan Plan) Evaluate(distFunc ik.SegmentMetric) (totalCost float64) {
	if len(plan) < 2 {
		return math.Inf(1)
	}
	last := map[string][]referenceframe.Input{}
	for i := 0; i < len(plan); i++ {
		for component, inputs := range plan[i] {
			if len(inputs) > 0 {
				if lastInputs, ok := last[component]; ok {
					cost := distFunc(&ik.Segment{StartConfiguration: lastInputs, EndConfiguration: inputs})
					totalCost += cost
				}
				last[component] = inputs
			}
		}
	}
	return totalCost
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

	return spatialmath.NewPose(goalPt, goalOrientation)
}

func stepsToNodes(steps [][]referenceframe.Input) []node {
	nodes := make([]node, 0, len(steps))
	for _, step := range steps {
		nodes = append(nodes, &basicNode{q: step})
	}
	return nodes
}

type resultPromise struct {
	steps  [][]referenceframe.Input
	future chan *rrtPlanReturn
}

func (r *resultPromise) result(ctx context.Context) ([][]referenceframe.Input, error) {
	if r.steps != nil && len(r.steps) > 0 {
		return r.steps, nil
	}
	// wait for a context cancel or a valid channel result
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		select {
		case planReturn := <-r.future:
			if planReturn.err() != nil {
				return nil, planReturn.err()
			}
			return nodesToInputs(planReturn.steps), nil
		default:
		}
	}
}
