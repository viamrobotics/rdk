package motionplan

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/golang/geo/r3"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan/ik"
	frame "go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestIKTolerances(t *testing.T) {
	logger := logging.NewTestLogger(t)

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("referenceframe/testjson/ur5eDH.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("")
	fs.AddFrame(m, fs.World())
	mp, err := newCBiRRTMotionPlanner(fs, rand.New(rand.NewSource(1)), logger, newBasicPlannerOptions())
	test.That(t, err, test.ShouldBeNil)

	// Test inability to arrive at another position due to orientation
	goal := &PlanState{poses: frame.FrameSystemPoses{m.Name(): frame.NewPoseInFrame(
		frame.World,
		spatial.NewPoseFromProtobuf(&commonpb.Pose{X: -46, Y: 0, Z: 372, OX: -1.78, OY: -3.3, OZ: -1.11}),
	)}}
	seed := &PlanState{configuration: map[string][]frame.Input{m.Name(): frame.FloatsToInputs(make([]float64, 6))}}
	_, err = mp.plan(context.Background(), seed, goal)
	test.That(t, err, test.ShouldNotBeNil)

	// Now verify that setting tolerances to zero allows the same arm to reach that position
	opt := newBasicPlannerOptions()
	opt.goalMetricConstructor = ik.NewPositionOnlyMetric
	opt.SetMaxSolutions(50)
	mp, err = newCBiRRTMotionPlanner(fs, rand.New(rand.NewSource(1)), logger, opt)
	test.That(t, err, test.ShouldBeNil)
	_, err = mp.plan(context.Background(), seed, goal)
	test.That(t, err, test.ShouldBeNil)
}

func TestConstraintPath(t *testing.T) {
	homePos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 0})
	toPos := frame.FloatsToInputs([]float64{0, 0, 0, 0, 0, 1})

	modelXarm, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/example_kinematics/xarm6_kinematics_test.json"), "")

	test.That(t, err, test.ShouldBeNil)
	ci := &ik.Segment{StartConfiguration: homePos, EndConfiguration: toPos, Frame: modelXarm}
	err = resolveSegmentsToPositions(ci)
	test.That(t, err, test.ShouldBeNil)

	handler := &ConstraintHandler{}

	// No constraints, should pass
	ok, failCI := handler.CheckSegmentAndStateValidity(ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	// Test interpolating with a proportional constraint, should pass
	constraint, _ := NewProportionalLinearInterpolatingConstraint(ci.StartPosition, ci.EndPosition, 0.01, 0.01)
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

	fs := frame.NewEmptyFrameSystem("test")

	m, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/example_kinematics/xarm7_kinematics_test.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerFrame, err := frame.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	goalFrame := fs.World()

	opt := newBasicPlannerOptions()
	startCfg := map[string][]frame.Input{m.Name(): m.InputFromProtobuf(mp1)}
	from := frame.FrameSystemPoses{markerFrame.Name(): frame.NewPoseInFrame(markerFrame.Name(), p1)}
	to := frame.FrameSystemPoses{markerFrame.Name(): frame.NewPoseInFrame(goalFrame.Name(), p2)}

	validFunc, gradFunc, err := CreateLineConstraintFS(fs, startCfg, from, to, 0.001)
	test.That(t, err, test.ShouldBeNil)

	_, innerGradFunc := NewLineConstraint(p1.Point(), p2.Point(), 0.001)
	pointGrad := innerGradFunc(&ik.State{Position: query})
	test.That(t, pointGrad, test.ShouldBeLessThan, 0.001*0.001)

	opt.SetPathMetric(gradFunc)
	opt.AddStateFSConstraint("whiteboard", validFunc)

	// This tests that we are able to advance partway, but not entirely, to the goal while keeping constraints, and return the last good
	// partway position
	ok, lastGood := opt.CheckSegmentAndStateValidityFS(
		&ik.SegmentFS{
			StartConfiguration: map[string][]frame.Input{m.Name(): m.InputFromProtobuf(mp1)},
			EndConfiguration:   map[string][]frame.Input{m.Name(): m.InputFromProtobuf(mp2)},
			FS:                 fs,
		},
		0.001,
	)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, lastGood, test.ShouldNotBeNil)
	// lastGood.StartConfiguration and EndConfiguration should pass constraints
	stateCheck := &ik.StateFS{Configuration: lastGood.StartConfiguration, FS: fs}
	pass, _ := opt.CheckStateFSConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeTrue)

	stateCheck.Configuration = lastGood.EndConfiguration
	pass, _ = opt.CheckStateFSConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeTrue)

	// Check that a deviating configuration will fail
	stateCheck.Configuration = map[string][]frame.Input{m.Name(): m.InputFromProtobuf(mpFail)}
	pass, failName := opt.CheckStateFSConstraints(stateCheck)
	test.That(t, pass, test.ShouldBeFalse)
	test.That(t, failName, test.ShouldStartWith, "whiteboard")
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
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/example_kinematics/xarm6_kinematics_test.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(frame.World))
	test.That(t, err, test.ShouldBeNil)
	seedMap := frame.NewZeroInputs(fs)
	handler := &ConstraintHandler{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(t, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := frame.FrameSystemGeometries(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if name != model.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(t, err, test.ShouldBeNil)

	_, collisionConstraints, err := createAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, nil,
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)
	for name, constraint := range collisionConstraints {
		handler.AddStateConstraint(name, constraint)
	}

	// loop through cases and check constraint handler processes them correctly
	for i, c := range cases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			response, failName := handler.CheckStateConstraints(&ik.State{Configuration: c.input, Frame: model})
			test.That(t, response, test.ShouldEqual, c.expected)
			test.That(t, failName, test.ShouldStartWith, c.failName)
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
	model, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/example_kinematics/xarm6_kinematics_test.json"), "")
	test.That(b, err, test.ShouldBeNil)
	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(frame.World))
	test.That(b, err, test.ShouldBeNil)
	seedMap := frame.NewZeroInputs(fs)
	handler := &ConstraintHandler{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(b, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := frame.FrameSystemGeometries(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)
	for name, geometries := range frameSystemGeometries {
		if name != model.Name() {
			staticRobotGeometries = append(staticRobotGeometries, geometries.Geometries()...)
		}
	}

	// Note that all obstacles in worldState are assumed to be static so it is ok to transform them into the world frame
	// TODO(rb) it is bad practice to assume that the current inputs of the robot correspond to the passed in world state
	// the state that observed the worldState should ultimately be included as part of the worldState message
	worldGeometries, err := worldState.ObstaclesInWorldFrame(fs, seedMap)
	test.That(b, err, test.ShouldBeNil)

	_, collisionConstraints, err := createAllCollisionConstraints(
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, nil,
		defaultCollisionBufferMM,
	)
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

func TestConstraintConstructors(t *testing.T) {
	c := NewEmptyConstraints()

	desiredLinearTolerance := float64(1000.0)
	desiredOrientationTolerance := float64(0.0)

	c.AddLinearConstraint(LinearConstraint{
		LineToleranceMm:          desiredLinearTolerance,
		OrientationToleranceDegs: desiredOrientationTolerance,
	})

	test.That(t, len(c.LinearConstraint), test.ShouldEqual, 1)
	test.That(t, c.LinearConstraint[0].LineToleranceMm, test.ShouldEqual, desiredLinearTolerance)
	test.That(t, c.LinearConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddOrientationConstraint(OrientationConstraint{
		OrientationToleranceDegs: desiredOrientationTolerance,
	})
	test.That(t, len(c.OrientationConstraint), test.ShouldEqual, 1)
	test.That(t, c.OrientationConstraint[0].OrientationToleranceDegs, test.ShouldEqual, desiredOrientationTolerance)

	c.AddCollisionSpecification(CollisionSpecification{
		Allows: []CollisionSpecificationAllowedFrameCollisions{
			{
				Frame1: "frame1",
				Frame2: "frame2",
			},
			{
				Frame1: "frame3",
				Frame2: "frame4",
			},
		},
	})
	test.That(t, len(c.CollisionSpecification), test.ShouldEqual, 1)
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame1, test.ShouldEqual, "frame1")
	test.That(t, c.CollisionSpecification[0].Allows[0].Frame2, test.ShouldEqual, "frame2")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame1, test.ShouldEqual, "frame3")
	test.That(t, c.CollisionSpecification[0].Allows[1].Frame2, test.ShouldEqual, "frame4")

	pbConstraint := c.ToProtobuf()
	pbToRDKConstraint := ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)
}
