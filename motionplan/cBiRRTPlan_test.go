package motionplan

import (
	"context"
	"math/rand"
	"sort"

	"testing"

	commonpb "go.viam.com/core/proto/api/common/v1"
	frame "go.viam.com/core/referenceframe"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"go.viam.com/test"
)

var interp = frame.FloatsToInputs([]float64{
	0.22034293025523666,
	0.023301860367034785,
	0.0035938741832804775,
	0.03706780636626979,
	-0.006010542176591475,
	0.013764993693680328,
	0.22994099248696265,
})

// This should test a simple linear motion
func TestSimpleLinearMotion(t *testing.T) {
	nSolutions := 5
	inputSteps := []*solution{}
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	m, err := frame.ParseJSONFile(utils.ResolveFile("robots/xarm/xArm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	ik, err := CreateCombinedIKSolver(m, logger, nCPU)
	test.That(t, err, test.ShouldBeNil)
	nlopt, err := CreateNloptIKSolver(m, logger)
	test.That(t, err, test.ShouldBeNil)
	// nlopt should try only once
	nlopt.SetMaxIter(1)
	mp := &cBiRRTMotionPlanner{solver: ik, fastGradDescent: nlopt, frame: m, logger: logger, solDist: 0.0001}

	// Max individual step of 0.5% of full range of motion
	mp.qstep = getFrameSteps(m, 0.015)
	mp.iter = 2000
	mp.stepSize = 1

	mp.randseed = rand.New(rand.NewSource(42))

	mp.opt = NewDefaultPlannerOptions()

	pos := &commonpb.Pose{
		X:  206,
		Y:  100,
		Z:  120.5,
		OY: -1,
	}
	corners := map[*solution]bool{}

	solutions, err := getSolutions(ctx, mp.opt, mp.solver, pos, home7, mp)
	test.That(t, err, test.ShouldBeNil)

	near1 := &solution{home7}
	seedMap := make(map[*solution]*solution)
	seedMap[near1] = nil
	target := &solution{interp}

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	goalMap := make(map[*solution]*solution)

	if len(keys) < nSolutions {
		nSolutions = len(keys)
	}

	for _, k := range keys[:nSolutions] {
		goalMap[&solution{solutions[k]}] = nil
	}

	// Extend tree seedMap as far towards target as it can get. It may or may not reach it.
	seedReached := mp.constrainedExtend(mp.opt, seedMap, near1, target)
	// Find the nearest point in goalMap to the furthest point reached in seedMap
	near2 := mp.nn.nearestNeighbor(ctx, seedReached, goalMap)
	// extend goalMap towards the point in seedMap
	goalReached := mp.constrainedExtend(mp.opt, goalMap, near2, seedReached)

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
	finalSteps := mp.SmoothPath(ctx, mp.opt, inputSteps, corners)
	test.That(t, len(finalSteps), test.ShouldBeLessThanOrEqualTo, unsmoothLen)
}

func TestNearestNeighbor(t *testing.T) {
	nm := &neighborManager{nCPU: 2}
	rrtMap := map[*solution]*solution{}

	j := &solution{[]frame.Input{{0.0}}}
	for i := 1.0; i < 110.0; i++ {
		iSol := &solution{[]frame.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	ctx := context.Background()

	seed := &solution{[]frame.Input{{23.1}}}
	// test serial NN
	nn := nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 23.0)

	for i := 120.0; i < 1100.0; i++ {
		iSol := &solution{[]frame.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = &solution{[]frame.Input{{723.6}}}
	// test parallel NN
	nn = nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 724.0)
}
