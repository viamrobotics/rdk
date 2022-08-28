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
	inputSteps := []*node{}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)
	nlopt, err := CreateNloptIKSolver(m, logger, 1)
	test.That(t, err, test.ShouldBeNil)
	// nlopt should try only once
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: m, logger: logger}

	mp.randseed = rand.New(rand.NewSource(42))

	opt := NewBasicPlannerOptions()

	pos := &commonpb.Pose{
		X:  206,
		Y:  100,
		Z:  120.5,
		OY: -1,
	}
	corners := map[*node]bool{}

	solutions, err := getSolutions(ctx, opt, mp.solver, pos, home7, mp.Frame())
	test.That(t, err, test.ShouldBeNil)

	near1 := &node{q: home7}
	seedMap := make(map[*node]*node)
	seedMap[near1] = nil
	target := interp

	goalMap := make(map[*node]*node)

	if len(solutions) < nSolutions {
		nSolutions = len(solutions)
	}

	for _, solution := range solutions[:nSolutions] {
		goalMap[solution] = nil
	}
	nn := &neighborManager{nCPU: nCPU}

	cOpt, err := newCbirrtOptions(opt, m)
	test.That(t, err, test.ShouldBeNil)

	// Extend tree seedMap as far towards target as it can get. It may or may not reach it.
	seedReached := mp.constrainedExtend(ctx, cOpt, seedMap, near1, &node{q: target})
	// Find the nearest point in goalMap to the furthest point reached in seedMap
	near2 := nn.nearestNeighbor(ctx, seedReached.q, goalMap)
	// extend goalMap towards the point in seedMap
	goalReached := mp.constrainedExtend(ctx, cOpt, goalMap, near2, seedReached)

	test.That(t, inputDist(seedReached.q, goalReached.q) < cOpt.JointSolveDist, test.ShouldBeTrue)

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
	finalSteps := mp.SmoothPath(ctx, cOpt, inputSteps, corners)
	test.That(t, len(finalSteps), test.ShouldBeLessThanOrEqualTo, unsmoothLen)
}
