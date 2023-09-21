package motionplan

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan/ik"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKTolerances(t *testing.T) {
	logger := golog.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("referenceframe/testjson/ur5eDH.json"), "")
	test.That(t, err, test.ShouldBeNil)
	mp, err := newCBiRRTMotionPlanner(m, rand.New(rand.NewSource(1)), logger, newBasicPlannerOptions(m))
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	pos := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  -46,
		Y:  0,
		Z:  372,
		OX: -1.78,
		OY: -3.3,
		OZ: -1.11,
	})
	_, err = mp.plan(context.Background(), pos, frame.FloatsToInputs([]float64{0, 0}))
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := newBasicPlannerOptions(m)
	opt.goalMetricConstructor = ik.NewPositionOnlyMetric
	opt.SetMaxSolutions(50)
	mp, err = newCBiRRTMotionPlanner(m, rand.New(rand.NewSource(1)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	_, err = mp.plan(context.Background(), pos, frame.FloatsToInputs(make([]float64, 6)))
	test.That(t, err, test.ShouldBeNil)
}

func TestConstraintPath(t *testing.T) {
	homePos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	toPos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 1})

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")

	test.That(t, err, test.ShouldBeNil)
	ci := &ik.Segment{StartConfiguration: homePos, EndConfiguration: toPos, Frame: modelXarm}
	err = resolveSegmentsToPositions(ci)
	test.That(t, err, test.ShouldBeNil)

	handler := &ConstraintHandler{}

	// No constraints, should pass
	ok, failCI := handler.CheckSegmentAndStateValidity(ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	// Test interpolating
	constraint, _ := NewProportionalLinearInterpolatingConstraint(ci.StartPosition, ci.EndPosition, 0.01)
	handler.AddStateConstraint("interp", constraint)
	ok, failCI = handler.CheckSegmentAndStateValidity(ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(handler.StateConstraints()), test.ShouldEqual, 1)

	badInterpPos := frame.FloatsToInputs([]float64{6.2, 0, 0, 0, 0, 0})
	ciBad := &ik.Segment{StartConfiguration: homePos, EndConfiguration: badInterpPos, Frame: modelXarm}
	err = resolveSegmentsToPositions(ciBad)
	test.That(t, err, test.ShouldBeNil)
	ok, failCI = handler.CheckSegmentAndStateValidity(ciBad, 0.5)
	test.That(t, failCI, test.ShouldNotBeNil) // With linear constraint, should be valid at the first step
	test.That(t, ok, test.ShouldBeFalse)
}

func TestLineFollow(t *testing.T) {
	p1 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  440,
		Y:  -447,
		Z:  500,
		OY: -1,
	})
	p2 := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  140,
		Y:  -447,
		Z:  550,
		OY: -1,
	})
	mp1 := frame.JointPositionsFromRadians([]float64{
		3.75646398939225,
		-1.0162453766159272,
		1.2142890600914453,
		1.0521227724322786,
		-0.21337105357552288,
		-0.006502311329196852,
		-4.3822913510408945,
	})
	mp2 := frame.JointPositionsFromRadians([]float64{
		3.896845654143853,
		-0.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	})
	mpFail := frame.JointPositionsFromRadians([]float64{
		3.896845654143853,
		-1.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	})

	query := spatial.NewPoseFromProtobuf(&commonpb.Pose{
		X:  289.94907586421124,
		Y:  -447,
		Z:  525.0086401700755,
		OY: -1,
	})

	validFunc, gradFunc := NewLineConstraint(p1.Point(), p2.Point(), 0.001)

	pointGrad := gradFunc(&ik.State{Position: query})
	test.That(t, pointGrad, test.ShouldBeLessThan, 0.001*0.001)

	fs := frame.NewEmptyFrameSystem("test")

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm7_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	goalFrame := fs.World()

	// Create a frame to solve for, and an IK solver with that frame.
	sf, err := newSolverFrame(fs, markerFrame.Name(), goalFrame.Name(), frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)

	opt := newBasicPlannerOptions(sf)
	opt.SetPathMetric(gradFunc)
	opt.AddStateConstraint("whiteboard", validFunc)

	ok, lastGood := opt.CheckSegmentAndStateValidity(
		&ik.Segment{
			StartConfiguration: sf.InputFromProtobuf(mp1),
			EndConfiguration:   sf.InputFromProtobuf(mp2),
			Frame:              sf,
		},
		1,
	)
	test.That(t, ok, test.ShouldBeFalse)
	// lastGood.StartConfiguration and EndConfiguration should pass constraints
	lastGood.Frame = sf
	stateCheck := &ik.State{Configuration: lastGood.StartConfiguration, Frame: lastGood.Frame}
	pass, _ := opt.CheckStateConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeTrue)

	stateCheck.Configuration = lastGood.EndConfiguration
	stateCheck.Position = nil
	pass, _ = opt.CheckStateConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeTrue)

	// Check that a deviating configuration will fail
	stateCheck.Configuration = sf.InputFromProtobuf(mpFail)
	stateCheck.Position = nil
	pass, failName := opt.CheckStateConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeFalse)
	test.That(t, failName, test.ShouldEqual, "whiteboard")
}

func TestCollisionConstraints(t *testing.T) {
	zeroPos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	cases := []struct {
		input    []frame.Input
		expected bool
		failName string
	}{
		{zeroPos, true, ""},
		{frame.FloatsToInputs([]float64{math.Pi / 2, 0, 0, 0, 0, 0}), true, ""},
		{frame.FloatsToInputs([]float64{math.Pi, 0, 0, 0, 0, 0}), false, defaultObstacleConstraintDesc},
		{frame.FloatsToInputs([]float64{math.Pi / 2, 0, 0, 0, 2, 0}), false, defaultSelfCollisionConstraintDesc},
	}

	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := frame.NewWorldState([]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, obstacles)}, nil)
	test.That(t, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, model.Name(), frame.World, frame.StartPositions(fs))
	test.That(t, err, test.ShouldBeNil)
	handler := &ConstraintHandler{}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, worldState, frame.StartPositions(fs), nil)
	test.That(t, err, test.ShouldBeNil)
	for name, constraint := range collisionConstraints {
		handler.AddStateConstraint(name, constraint)
	}

	// loop through cases and check constraint handler processes them correctly
	for i, c := range cases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			response, failName := handler.CheckStateConstraints(&ik.State{Configuration: c.input, Frame: model})
			test.That(t, response, test.ShouldEqual, c.expected)
			test.That(t, failName, test.ShouldEqual, c.failName)
		})
	}
}

var bt bool

func BenchmarkCollisionConstraints(b *testing.B) {
	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(b, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := frame.NewWorldState([]*frame.GeometriesInFrame{frame.NewGeometriesInFrame(frame.World, obstacles)}, nil)
	test.That(b, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(b, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(frame.World))
	test.That(b, err, test.ShouldBeNil)
	sf, err := newSolverFrame(fs, model.Name(), frame.World, frame.StartPositions(fs))
	test.That(b, err, test.ShouldBeNil)
	handler := &ConstraintHandler{}
	collisionConstraints, err := createAllCollisionConstraints(sf, fs, worldState, frame.StartPositions(fs), nil)
	test.That(b, err, test.ShouldBeNil)
	for name, constraint := range collisionConstraints {
		handler.AddStateConstraint(name, constraint)
	}
	rseed := rand.New(rand.NewSource(1))
	var b1 bool
	var n int

	// loop through cases and check constraint handler processes them correctly
	for n = 0; n < b.N; n++ {
		rfloats := frame.GenerateRandomConfiguration(model, rseed)
		b1, _ = handler.CheckStateConstraints(&ik.State{Configuration: frame.FloatsToInputs(rfloats), Frame: model})
	}
	bt = b1
}
