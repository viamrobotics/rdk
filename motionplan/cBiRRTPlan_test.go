package motionplan

import (
	"context"
	"math/rand"
	"sort"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func Test2DPlan(t *testing.T) {
	// Test Map:
	//      - bounds are from (-10, -10) to (10, 10)
	//      - obstacle from (-4, 4) to (4, 10)
	// ------------------------
	// | +      |    |      + |
	// |        |    |        |
	// |        |    |        |
	// |        |    |        |
	// |        ------        |
	// |          *           |
	// |                      |
	// |                      |
	// |                      |
	// ------------------------

	// setup problem parameters
	start := frame.FloatsToInputs([]float64{-9., 9.})
	goal := spatial.PoseToProtobuf(spatial.NewPoseFromPoint(r3.Vector{X: 9, Y: 9, Z: 0}))
	limits := []frame.Limit{{Min: -10, Max: 10}, {Min: -10, Max: 10}}

	// build obstacles
	obstacles := map[string]spatial.Geometry{}
	box, err := spatial.NewBox(spatial.NewPoseFromPoint(r3.Vector{0, 6, 0}), r3.Vector{8, 8, 1})
	test.That(t, err, test.ShouldBeNil)
	obstacles["box"] = box

	// build model
	physicalGeometry, err := spatial.NewBoxCreator(r3.Vector{X: 1, Y: 1, Z: 1}, spatial.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	model, err := frame.NewMobileFrame("mobile-base", limits, physicalGeometry)
	test.That(t, err, test.ShouldBeNil)

	// plan
	cbert, err := NewCBiRRTMotionPlanner(model, 1, logger)
	test.That(t, err, test.ShouldBeNil)
	opt := NewDefaultPlannerOptions()
	constraint := NewCollisionConstraintFromFrame(model, obstacles)
	test.That(t, err, test.ShouldBeNil)
	opt.AddConstraint("collision", constraint)
	waypoints, err := cbert.Plan(context.Background(), goal, start, opt)
	test.That(t, err, test.ShouldBeNil)

	obstacle := obstacles["box"]
	workspace, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{20, 20, 1})
	test.That(t, err, test.ShouldBeNil)
	for _, waypoint := range waypoints {
		pt := r3.Vector{waypoint[0].Value, waypoint[1].Value, 0}
		collides, err := obstacle.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, collides, test.ShouldBeFalse)
		inWorkspace, err := workspace.CollidesWith(spatial.NewPoint(pt))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, inWorkspace, test.ShouldBeTrue)
		logger.Debug("%f\t%f\n", pt.X, pt.Y)
	}
}

var interp = frame.FloatsToInputs([]float64{
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
	m, err := frame.ParseModelJSONFile(utils.ResolveFile("component/arm/xarm/xArm7_kinematics.json"), "")
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

	keys := make([]float64, 0, len(solutions))
	for k := range solutions {
		keys = append(keys, k)
	}
	sort.Float64s(keys)

	goalMap := make(map[*configuration]*configuration)

	if len(keys) < nSolutions {
		nSolutions = len(keys)
	}

	for _, k := range keys[:nSolutions] {
		goalMap[&configuration{solutions[k]}] = nil
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

func TestNearestNeighbor(t *testing.T) {
	nm := &neighborManager{nCPU: 2}
	rrtMap := map[*configuration]*configuration{}

	j := &configuration{[]referenceframe.Input{{0.0}}}
	for i := 1.0; i < 110.0; i++ {
		iSol := &configuration{[]referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	ctx := context.Background()

	seed := &configuration{[]referenceframe.Input{{23.1}}}
	// test serial NN
	nn := nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 23.0)

	for i := 120.0; i < 1100.0; i++ {
		iSol := &configuration{[]referenceframe.Input{{i}}}
		rrtMap[iSol] = j
		j = iSol
	}
	seed = &configuration{[]referenceframe.Input{{723.6}}}
	// test parallel NN
	nn = nm.nearestNeighbor(ctx, seed, rrtMap)
	test.That(t, nn.inputs[0].Value, test.ShouldAlmostEqual, 724.0)
}
