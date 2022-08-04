package motionplan

import (
	"context"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/utils"
)

var interp = referenceframe.FloatsToInputs([]float64{
	0.22034293025523666,
	0.023301860367034785,
	0.0035938741832804775,
	0.03706780636626979,
	-0.006010542176591475,
	0.013764993693680328,
	0.22994099248696265,
})

// This should test a simple linear motion.
func TestSimpleLinearMotion(t *testing.T) {
	nSolutions := 5
	inputSteps := []*configuration{}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)
	nlopt, err := CreateNloptIKSolver(m, logger, 1)
	test.That(t, err, test.ShouldBeNil)
	// nlopt should try only once
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: m, logger: logger, solDist: 0.0001}

	// Max individual step of 0.5% of full range of motion
	mp.qstep = getFrameSteps(m, 0.015)
	mp.iter = 2000
	mp.stepSize = 1

	mp.randseed = rand.New(rand.NewSource(42))

	opt := NewDefaultPlannerOptions()

	pos := &commonpb.Pose{
		X:  206,
		Y:  100,
		Z:  120.5,
		OY: -1,
	}
	corners := map[*configuration]bool{}

	solutions, err := getSolutions(ctx, opt, mp.solver, pos, home7, mp.Frame())
	test.That(t, err, test.ShouldBeNil)

	near1 := &configuration{home7}
	seedMap := make(map[*configuration]*configuration)
	seedMap[near1] = nil
	target := &configuration{interp}

	goalMap := make(map[*configuration]*configuration)

	if len(solutions) < nSolutions {
		nSolutions = len(solutions)
	}

	for _, solution := range solutions[:nSolutions] {
		goalMap[&configuration{solution}] = nil
	}
	nn := &neighborManager{nCPU: nCPU}

	// Extend tree seedMap as far towards target as it can get. It may or may not reach it.
	seedReached := mp.constrainedExtend(ctx, opt, seedMap, near1, target)
	// Find the nearest point in goalMap to the furthest point reached in seedMap
	near2 := nn.nearestNeighbor(ctx, seedReached, goalMap)
	// extend goalMap towards the point in seedMap
	goalReached := mp.constrainedExtend(ctx, opt, goalMap, near2, seedReached)

	test.That(t, inputDist(seedReached.inputs, goalReached.inputs) < mp.solDist, test.ShouldBeTrue)

	corners[seedReached] = true
	corners[goalReached] = true

	// extract the path to the seed
	for seedReached != nil {
		inputSteps = append(inputSteps, seedReached)
		seedReached = seedMap[seedReached]
	}
	// reverse the slice
	for i, j := 0, len(inputSteps)-1; i < j; i, j = i+1, j-1 {
		inputSteps[i], inputSteps[j] = inputSteps[j], inputSteps[i]
	}
	// extract the path to the goal
	for goalReached != nil {
		inputSteps = append(inputSteps, goalReached)
		goalReached = goalMap[goalReached]
	}

	// Test that smoothing succeeds and does not lengthen the path (it may be the same length)
	unsmoothLen := len(inputSteps)
	finalSteps := mp.SmoothPath(ctx, opt, inputSteps, corners)
	test.That(t, len(finalSteps), test.ShouldBeLessThanOrEqualTo, unsmoothLen)
}
