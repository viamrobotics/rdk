package motionplan

import (
	"context"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
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
// This test will step through the different stages of cbirrt and test each one in turn.
func TestSimpleLinearMotion(t *testing.T) {
	nSolutions := 5
	inputSteps := []node{}
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	m, err := referenceframe.ParseModelJSONFile(rutils.ResolveFile("components/arm/example_kinematics/xarm7_kinematics_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	goalPos := spatialmath.NewPose(r3.Vector{X: 206, Y: 100, Z: 120.5}, &spatialmath.OrientationVectorDegrees{OY: -1})

	opt := newBasicPlannerOptions()
	goalMetric := opt.getGoalMetric(referenceframe.FrameSystemPoses{m.Name(): referenceframe.NewPoseInFrame(referenceframe.World, goalPos)})
	fs := referenceframe.NewEmptyFrameSystem("")
	fs.AddFrame(m, fs.World())
	mp, err := newCBiRRTMotionPlanner(fs, rand.New(rand.NewSource(42)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	cbirrt, _ := mp.(*cBiRRTMotionPlanner)
	solutions, err := mp.getSolutions(ctx, referenceframe.FrameSystemInputs{m.Name(): home7}, goalMetric)
	test.That(t, err, test.ShouldBeNil)

	near1 := &basicNode{q: referenceframe.FrameSystemInputs{m.Name(): home7}}
	seedMap := make(map[node]node)
	seedMap[near1] = nil
	target := referenceframe.FrameSystemInputs{m.Name(): interp}

	goalMap := make(map[node]node)

	if len(solutions) < nSolutions {
		nSolutions = len(solutions)
	}

	for _, solution := range solutions[:nSolutions] {
		goalMap[solution] = nil
	}
	nn := &neighborManager{nCPU: nCPU}

	_, err = newCbirrtOptions(opt, cbirrt.lfs)
	test.That(t, err, test.ShouldBeNil)

	m1chan := make(chan node, 1)
	defer close(m1chan)

	// Extend tree seedMap as far towards target as it can get. It may or may not reach it.
	utils.PanicCapturingGo(func() {
		cbirrt.constrainedExtend(ctx, cbirrt.randseed, seedMap, near1, &basicNode{q: target}, m1chan)
	})
	seedReached := <-m1chan
	// Find the nearest point in goalMap to the furthest point reached in seedMap
	near2 := nn.nearestNeighbor(ctx, opt, seedReached, goalMap)
	// extend goalMap towards the point in seedMap
	utils.PanicCapturingGo(func() {
		cbirrt.constrainedExtend(ctx, cbirrt.randseed, goalMap, near2, seedReached, m1chan)
	})
	goalReached := <-m1chan
	dist := opt.configurationDistanceFunc(&ik.SegmentFS{StartConfiguration: seedReached.Q(), EndConfiguration: goalReached.Q()})
	test.That(t, dist < cbirrt.planOpts.InputIdentDist, test.ShouldBeTrue)

	seedReached.SetCorner(true)
	goalReached.SetCorner(true)

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
	finalSteps := cbirrt.smoothPath(ctx, inputSteps)
	test.That(t, len(finalSteps), test.ShouldBeLessThanOrEqualTo, unsmoothLen)
	// Test that path has changed after smoothing was applied
	test.That(t, finalSteps, test.ShouldNotResemble, inputSteps)
}
