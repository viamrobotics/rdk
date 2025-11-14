package armplanning

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

// This should test a simple linear motion.
// This test will step through the different stages of cbirrt and test each one in turn.
func TestSimpleLinearMotion(t *testing.T) {
	nSolutions := 5
	inputSteps := []*node{}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/fake/kinematics/xarm7.json"), "")
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 206, Y: 100, Z: 120.5}, &spatialmath.OrientationVectorDegrees{OY: -1})

	opt := NewBasicPlannerOptions()
	goal := referenceframe.FrameSystemPoses{m.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goalPos)}
	fs := referenceframe.NewEmptyFrameSystem("")
	fs.AddFrame(m, fs.World())

	// Create PlanRequest to use the new API
	request := &PlanRequest{
		FrameSystem:    fs,
		Goals:          []*PlanState{NewPlanState(goal, nil)},
		StartState:     NewPlanState(nil, referenceframe.FrameSystemInputs{m.Name(): home7}),
		PlannerOptions: opt,
		Constraints:    &motionplan.Constraints{},
	}

	pc, err := newPlanContext(ctx, logger, request, &PlanMeta{})
	test.That(t, err, test.ShouldBeNil)

	psc, err := newPlanSegmentContext(ctx, pc, referenceframe.FrameSystemInputs{m.Name(): home7}.ToLinearInputs(), goal)
	test.That(t, err, test.ShouldBeNil)

	mp, err := newCBiRRTMotionPlanner(ctx, pc, psc, logger.Sublogger("cbirrt"))
	test.That(t, err, test.ShouldBeNil)
	solutions, err := getSolutions(ctx, psc, logger.Sublogger("ik"))
	test.That(t, err, test.ShouldBeNil)

	near1 := &node{inputs: referenceframe.FrameSystemInputs{m.Name(): home7}.ToLinearInputs()}
	seedMap := rrtMap{}
	seedMap[near1] = nil
	target := referenceframe.NewLinearInputs()
	target.Put(m.Name(), []referenceframe.Input{
		0.22034293025523666,
		0.023301860367034785,
		0.0035938741832804775,
		0.03706780636626979,
		-0.006010542176591475,
		0.013764993693680328,
		0.22994099248696265,
	})

	goalMap := rrtMap{}

	if len(solutions) < nSolutions {
		nSolutions = len(solutions)
	}

	for _, solution := range solutions[:nSolutions] {
		goalMap[solution] = nil
	}

	// Extend tree seedMap as far towards target as it can get. It may or may not reach it.
	seedReached := mp.constrainedExtend(ctx, 1, seedMap, near1, &node{inputs: target})

	// Find the nearest point in goalMap to the furthest point reached in seedMap
	near2 := nearestNeighbor(seedReached, goalMap, nodeConfigurationDistanceFunc)
	// extend goalMap towards the point in seedMap
	goalReached := mp.constrainedExtend(ctx, 1, goalMap, near2, seedReached)

	dist := pc.configurationDistanceFunc(
		&motionplan.SegmentFS{StartConfiguration: seedReached.inputs, EndConfiguration: goalReached.inputs},
	)
	test.That(t, dist, test.ShouldBeLessThan, pc.planOpts.InputIdentDist)

	seedReached.corner = true
	goalReached.corner = true

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
	// Convert node slice to FrameSystemInputs slice for smoothPath
	inputSlice := make([]*referenceframe.LinearInputs, len(inputSteps))
	for i, step := range inputSteps {
		inputSlice[i] = step.inputs
	}
	finalSteps := smoothPath(ctx, psc, inputSlice)
	test.That(t, len(finalSteps), test.ShouldBeLessThanOrEqualTo, unsmoothLen)
	// Test that path has changed after smoothing was applied
	test.That(t, finalSteps, test.ShouldNotResemble, inputSteps)
}
