package motionplan

import (
	"context"
	"errors"
	"math"

	"go.viam.com/core/kinematics"
	frame "go.viam.com/core/referenceframe"
	spatial "go.viam.com/core/spatialmath"
	"go.viam.com/core/utils"
)

// MotionPlanner defines a struct able to plan motion
type MotionPlanner interface {
	// Plan will take a context, a goal position, and an input start state and return a series of state waypoints which
	// should be visited in order to arrive at the goal while satisfying all constraints
	Plan(context.Context, spatial.Pose, []frame.Input) ([][]frame.Input, error)
	AddConstraint(string, func(constraintInput) bool)
	RemoveConstraint(string)
	Constraints() []string
}

// NewLinearMotionPlanner returns a linearMotionPlanner
func NewLinearMotionPlanner(ik kinematics.InverseKinematics, frame frame.Frame) MotionPlanner {
	return &linearMotionPlanner{solver: ik, frame: frame}
}

// A straightforward motion planner that will path a straight line from start to end
type linearMotionPlanner struct {
	constraintHandler
	solver  kinematics.InverseKinematics
	frame   frame.Frame
	validityCheck func([]frame.Input) bool // This can check for env collisions, self collisions, etc
}

func (mp *linearMotionPlanner) Plan(ctx context.Context, goalPos spatial.Pose, seed []frame.Input) ([][]frame.Input, error) {
	var inputSteps [][]frame.Input

	seedPos, err := mp.frame.Transform(seed)
	if err != nil {
		return nil, err
	}

	// First, we break down the spatial distance and rotational distance from seed to goal, and determine the number
	// of steps needed to get from one to the other
	nSteps := mp.getSteps(seedPos, goalPos)

	// Intermediate pos for constraint checking
	lastPos := seedPos

	// Create the required steps. nSteps is guaranteed to be at least 1.
	for i := 1; i < nSteps; i++ {
		intPos := spatial.Interpolate(seedPos, goalPos, float64(i)/float64(nSteps))

		cPass := false
		nTries := 30
		// TODO: make it easy to request additional solutions from IK without re-initializing
		var step []frame.Input
		for !cPass {
			if nTries < 0 {
				return nil, errors.New("could not solve path within constraints")
			}
			step, err = mp.solver.Solve(ctx, spatial.PoseToArmPos(intPos), seed)
			if err != nil {
				return nil, err
			}
			cPass = mp.checkConstraints(constraintInput{
				lastPos,
				intPos,
				seed,
				step,
				mp.frame})
			nTries--
		}

		if mp.validityCheck != nil {
			if !mp.validityCheck(step) {
				// TODO: Do thing to get around obstruction
				return nil, errors.New("path reached invalid state")
			}
		}
		lastPos = intPos
		seed = step
		// Append deep copy of result to inputSteps
		inputSteps = append(inputSteps, append([]frame.Input{}, step...))
	}

	return inputSteps, nil
}

func (mp *linearMotionPlanner) checkConstraints(cInput constraintInput) bool {
	for _, cFunc := range mp.constraints {
		if !cFunc(cInput) {
			return false
		}
	}
	return true
}

// getSteps will determine the number of steps which should be used to get from the seed to the goal.
// The returned value is guaranteed to be at least 1.
func (mp *linearMotionPlanner) getSteps(seedPos, goalPos spatial.Pose) int {
	maxLinear := 2.  // max mm movement per step
	maxDegrees := 2. // max R4AA degrees per step

	mmDist := seedPos.Point().Distance(goalPos.Point())
	rDist := spatial.OrientationBetween(seedPos.Orientation(), goalPos.Orientation()).AxisAngles()

	nSteps := math.Max(math.Abs(mmDist/maxLinear), math.Abs(utils.RadToDeg(rDist.Theta)/maxDegrees))
	return int(nSteps) + 1
}
