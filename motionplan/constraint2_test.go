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

	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestConstraintPath(t *testing.T) {
	ctx := context.Background()
	homePos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	toPos := []referenceframe.Input{0, 0, 0, 0, 0, 1}

	modelXarm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")

	test.That(t, err, test.ShouldBeNil)
	ci := &Segment{StartConfiguration: homePos, EndConfiguration: toPos, Frame: modelXarm}
	err = resolveSegmentsToPositions(ci)
	test.That(t, err, test.ShouldBeNil)

	handler := &ConstraintChecker{}

	// No constraints, should pass
	ok, failCI := handler.CheckSegmentAndStateValidity(ctx, ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	// Test interpolating with a proportional constraint, should pass
	constraint, _ := newProportionalLinearInterpolatingConstraint(ci.StartPosition, ci.EndPosition, 0.01, 0.01)
	handler.AddStateConstraint("interp", constraint)
	ok, failCI = handler.CheckSegmentAndStateValidity(ctx, ci, 0.5)
	test.That(t, failCI, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	test.That(t, len(handler.StateConstraints()), test.ShouldEqual, 1)

	badInterpPos := []referenceframe.Input{6.2, 0, 0, 0, 0, 0}
	ciBad := &Segment{StartConfiguration: homePos, EndConfiguration: badInterpPos, Frame: modelXarm}
	err = resolveSegmentsToPositions(ciBad)
	test.That(t, err, test.ShouldBeNil)
	ok, failCI = handler.CheckSegmentAndStateValidity(ctx, ciBad, 0.5)
	test.That(t, failCI, test.ShouldNotBeNil) // With linear constraint, should be valid at the first step
	test.That(t, ok, test.ShouldBeFalse)
}

func TestLineFollow(t *testing.T) {
	ctx := context.Background()
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
	mp1 := referenceframe.JointPositionsFromRadians([]float64{
		3.75646398939225,
		-1.0162453766159272,
		1.2142890600914453,
		1.0521227724322786,
		-0.21337105357552288,
		-0.006502311329196852,
		-4.3822913510408945,
	})
	mp2 := referenceframe.JointPositionsFromRadians([]float64{
		3.896845654143853,
		-0.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	})
	mpFail := referenceframe.JointPositionsFromRadians([]float64{
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

	fs := referenceframe.NewEmptyFrameSystem("test")

	m, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm7.json"), "")
	test.That(t, err, test.ShouldBeNil)

	err = fs.AddFrame(m, fs.World())
	test.That(t, err, test.ShouldBeNil)

	markerFrame, err := referenceframe.NewStaticFrame("marker", spatial.NewPoseFromPoint(r3.Vector{0, 0, 105}))
	test.That(t, err, test.ShouldBeNil)
	err = fs.AddFrame(markerFrame, m)
	test.That(t, err, test.ShouldBeNil)
	goalFrame := fs.World()

	opt := NewEmptyConstraintChecker()
	startCfg := referenceframe.FrameSystemInputs{m.Name(): m.InputFromProtobuf(mp1)}.ToLinearInputs()
	from := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(markerFrame.Name(), p1)}
	to := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(goalFrame.Name(), p2)}

	constructor := func(fromPose, toPose spatial.Pose, tolerance float64) (StateConstraint, StateMetric) {
		return NewLineConstraint(fromPose.Point(), toPose.Point(), .001)
	}
	constraintInternal, err := newFsPathConstraintTol(fs, startCfg, from, to, constructor, .001)
	test.That(t, err, test.ShouldBeNil)

	validFunc := constraintInternal.constraint
	gradFunc := constraintInternal.metric

	_, innerGradFunc := NewLineConstraint(p1.Point(), p2.Point(), 0.001)
	pointGrad := innerGradFunc(&State{Position: query})
	test.That(t, pointGrad, test.ShouldBeLessThan, 0.001*0.001)

	opt.pathMetric = gradFunc
	opt.AddStateFSConstraint("whiteboard", validFunc)

	// This tests that we are able to advance partway, but not entirely, to the goal while keeping constraints, and return the last good
	// partway position
	lastGood, err := opt.CheckSegmentAndStateValidityFS(
		ctx,
		&SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{m.Name(): m.InputFromProtobuf(mp1)}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{m.Name(): m.InputFromProtobuf(mp2)}.ToLinearInputs(),
			FS:                 fs,
		},
		0.001,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, lastGood, test.ShouldNotBeNil)
	// lastGood.StartConfiguration and EndConfiguration should pass constraints
	stateCheck := &StateFS{Configuration: lastGood.StartConfiguration, FS: fs}
	test.That(t, opt.CheckStateFSConstraints(ctx, stateCheck), test.ShouldBeNil)

	stateCheck.Configuration = lastGood.EndConfiguration
	test.That(t, opt.CheckStateFSConstraints(ctx, stateCheck), test.ShouldBeNil)

	// Check that a deviating configuration will fail
	stateCheck.Configuration = referenceframe.FrameSystemInputs{m.Name(): m.InputFromProtobuf(mpFail)}.ToLinearInputs()
	err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldStartWith, "whiteboard")
}

func TestCollisionConstraints(t *testing.T) {
	zeroPos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	cases := []struct {
		input    []referenceframe.Input
		expected bool
		failName string
	}{
		{zeroPos, true, ""},
		{[]referenceframe.Input{math.Pi / 2, 0, 0, 0, 0, 0}, true, ""},
		{[]referenceframe.Input{math.Pi, 0, 0, 0, 0, 0}, false, obstacleConstraintDescription},
		{[]referenceframe.Input{math.Pi / 2, 0, 0, 0, 2, 0}, false, selfCollisionConstraintDescription},
	}

	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
	}, nil)
	test.That(t, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(referenceframe.World))
	test.That(t, err, test.ShouldBeNil)
	seedMap := referenceframe.NewZeroInputs(fs)
	handler := &ConstraintChecker{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(t, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
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

	_, collisionConstraints, err := CreateAllCollisionConstraints(
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
			err := handler.CheckStateConstraints(&State{Configuration: c.input, Frame: model})
			test.That(t, err == nil, test.ShouldEqual, c.expected)
			if err != nil {
				test.That(t, err.Error(), test.ShouldStartWith, c.failName)
			}
		})
	}
}

func BenchmarkCollisionConstraints(b *testing.B) {
	// define external obstacles
	bc, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(b, err, test.ShouldBeNil)
	obstacles := []spatial.Geometry{}
	obstacles = append(obstacles, bc.Transform(spatial.NewZeroPose()))
	obstacles = append(obstacles, bc.Transform(spatial.NewPoseFromPoint(r3.Vector{-130, 0, 300})))
	worldState, err := referenceframe.NewWorldState([]*referenceframe.GeometriesInFrame{
		referenceframe.NewGeometriesInFrame(referenceframe.World, obstacles),
	}, nil)
	test.That(b, err, test.ShouldBeNil)

	// setup zero position as reference CollisionGraph and use it in handler
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(b, err, test.ShouldBeNil)
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.Frame(referenceframe.World))
	test.That(b, err, test.ShouldBeNil)
	seedMap := referenceframe.NewZeroInputs(fs)
	handler := &ConstraintChecker{}

	// create robot collision entities
	movingGeometriesInFrame, err := model.Geometries(seedMap[model.Name()])
	movingRobotGeometries := movingGeometriesInFrame.Geometries()
	test.That(b, err, test.ShouldBeNil)

	// find all geometries that are not moving but are in the frame system
	staticRobotGeometries := make([]spatial.Geometry, 0)
	frameSystemGeometries, err := referenceframe.FrameSystemGeometries(fs, seedMap)
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

	_, collisionConstraints, err := CreateAllCollisionConstraints(
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

	// loop through cases and check constraint handler processes them correctly
	for n := 0; n < b.N; n++ {
		rfloats := referenceframe.GenerateRandomConfiguration(model, rseed)
		err = handler.CheckStateConstraints(&State{Configuration: rfloats, Frame: model})
		test.That(b, err, test.ShouldBeNil)
	}
}
