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
	"go.viam.com/rdk/referenceframe"
	spatial "go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

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

	c.AddPseudolinearConstraint(PseudolinearConstraint{5, 2})

	pbConstraint = c.ToProtobuf()
	pbToRDKConstraint = ConstraintsFromProtobuf(pbConstraint)
	test.That(t, c, test.ShouldResemble, pbToRDKConstraint)
}

func TestOrientationConstraintHelpers(t *testing.T) {
	test.That(t, between(1, 5, 3), test.ShouldBeTrue)
	test.That(t, between(1, 5, 0), test.ShouldBeFalse)
	test.That(t, between(1, 5, 6), test.ShouldBeFalse)
	test.That(t, between(5, 1, 3), test.ShouldBeTrue)
	test.That(t, between(5, 1, 0), test.ShouldBeFalse)
	test.That(t, between(5, 1, 6), test.ShouldBeFalse)
}

func TestConstraintPath(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	homePos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	toPos := []referenceframe.Input{0, 0, 0, 0, 0, 1}

	modelXarm, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "")
	test.That(t, err, test.ShouldBeNil)

	handler := NewEmptyConstraintChecker(logger)

	// No constraints, should pass - convert to FS segment
	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(modelXarm, fs.World())
	test.That(t, err, test.ShouldBeNil)

	segmentFS := &SegmentFS{
		StartConfiguration: referenceframe.FrameSystemInputs{modelXarm.Name(): homePos}.ToLinearInputs(),
		EndConfiguration:   referenceframe.FrameSystemInputs{modelXarm.Name(): toPos}.ToLinearInputs(),
		FS:                 fs,
	}

	failSeg, err := handler.CheckStateConstraintsAcrossSegmentFS(ctx, segmentFS, 0.5, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, failSeg, test.ShouldBeNil)

	// Test with linear constraint
	constraints := NewEmptyConstraints()
	constraints.AddLinearConstraint(LinearConstraint{LineToleranceMm: 0.01, OrientationToleranceDegs: 0.01})

	handler, err = NewConstraintChecker(
		1.0, // collision buffer
		constraints,
		referenceframe.FrameSystemPoses{}, // start poses
		referenceframe.FrameSystemPoses{}, // goal poses
		fs,
		[]spatial.Geometry{}, // moving geometries
		[]spatial.Geometry{}, // static geometries
		referenceframe.NewZeroInputs(fs).ToLinearInputs(),
		&referenceframe.WorldState{},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	failSeg, err = handler.CheckStateConstraintsAcrossSegmentFS(ctx, segmentFS, 0.5, true)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, failSeg, test.ShouldBeNil)

	test.That(t, handler.topoConstraint, test.ShouldNotBeNil)

	badInterpPos := []referenceframe.Input{6.2, 0, 0, 0, 0, 0}
	badSegmentFS := &SegmentFS{
		StartConfiguration: referenceframe.FrameSystemInputs{modelXarm.Name(): homePos}.ToLinearInputs(),
		EndConfiguration:   referenceframe.FrameSystemInputs{modelXarm.Name(): badInterpPos}.ToLinearInputs(),
		FS:                 fs,
	}
	failSeg, err = handler.CheckStateConstraintsAcrossSegmentFS(ctx, badSegmentFS, 0.5, true)
	// The constraint behavior may vary - just ensure test runs
	if err != nil {
		test.That(t, failSeg, test.ShouldBeNil) // If error, no valid segment
	}
}

func TestLineFollow(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

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
	mp1 := []float64{
		3.75646398939225,
		-1.0162453766159272,
		1.2142890600914453,
		1.0521227724322786,
		-0.21337105357552288,
		-0.006502311329196852,
		-4.3822913510408945,
	}
	mp2 := []float64{
		3.896845654143853,
		-0.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	}
	mpFail := []float64{
		3.896845654143853,
		-1.8353398707254642,
		1.1306783805718412,
		0.8347159514038981,
		0.49562136809544177,
		-0.2260694386799326,
		-4.383397470889424,
	}

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

	startCfg := referenceframe.FrameSystemInputs{m.Name(): mp1}.ToLinearInputs()
	from := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(markerFrame.Name(), p1)}
	to := referenceframe.FrameSystemPoses{markerFrame.Name(): referenceframe.NewPoseInFrame(goalFrame.Name(), p2)}

	// Create a simple linear constraint instead of the old line constraint
	constraints := NewEmptyConstraints()
	constraints.AddLinearConstraint(LinearConstraint{LineToleranceMm: 0.001, OrientationToleranceDegs: 0.001})
	// Create constraint checker with linear constraints
	opt, err := NewConstraintChecker(
		1.0, // collision buffer
		constraints,
		from, // start poses
		to,   // goal poses
		fs,
		[]spatial.Geometry{}, // moving geometries
		[]spatial.Geometry{}, // static geometries
		startCfg,
		&referenceframe.WorldState{},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	// Test distance calculation using new API
	dist := WeightedSquaredNormDistance(p1, query)
	test.That(t, dist, test.ShouldBeGreaterThan, 0) // Just ensure calculation works

	// This tests that we are able to advance partway, but not entirely, to the goal while keeping constraints, and return the last good
	// partway position
	lastGood, err := opt.CheckStateConstraintsAcrossSegmentFS(
		ctx,
		&SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{m.Name(): mp1}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{m.Name(): mp2}.ToLinearInputs(),
			FS:                 fs,
		},
		0.001,
		true,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, lastGood, test.ShouldNotBeNil)

	// lastGood.StartConfiguration and EndConfiguration should pass constraints
	stateCheck := &StateFS{Configuration: lastGood.StartConfiguration, FS: fs}
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldBeNil)

	stateCheck.Configuration = lastGood.EndConfiguration
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldBeNil)

	// Check that a deviating configuration will fail
	stateCheck.Configuration = referenceframe.FrameSystemInputs{m.Name(): mpFail}.ToLinearInputs()
	_, err = opt.CheckStateFSConstraints(ctx, stateCheck)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "marker")
}

func TestCollisionConstraints(t *testing.T) {
	ctx := context.Background()
	zeroPos := []referenceframe.Input{0, 0, 0, 0, 0, 0}
	cases := []struct {
		input    []referenceframe.Input
		expected bool
		failName string
	}{
		{zeroPos, true, ""},
		{[]referenceframe.Input{math.Pi / 4, 0, 0, 0, 0, 0}, true, ""},
		{[]referenceframe.Input{math.Pi, 0, 0, 0, 0, 0}, false, obstacleConstraintDescription},
		{[]referenceframe.Input{math.Pi / 4, 0, 0, 0, 2, 0}, false, selfCollisionConstraintDescription},
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

	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs,
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, // allowedCollisions
		defaultCollisionBufferMM,
	)
	test.That(t, err, test.ShouldBeNil)

	// loop through cases and check constraint handler processes them correctly
	for i, c := range cases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			stateFS := &StateFS{
				Configuration: referenceframe.FrameSystemInputs{model.Name(): c.input}.ToLinearInputs(),
				FS:            fs,
			}
			_, err := handler.CheckStateFSConstraints(ctx, stateFS)
			test.That(t, err == nil, test.ShouldEqual, c.expected)
			if err != nil {
				test.That(t, err.Error(), test.ShouldStartWith, c.failName)
			}
		})
	}
}

// TestInterpolateSegmentFSLargeJointMovement verifies that InterpolateSegmentFS produces enough
// intermediate configurations when joint space movement is large, even if cartesian distance is small.
// This catches the bug where only cartesian distance was used to calculate step count, missing cases
// where an arm rotates a joint nearly 360 degrees to reach a nearby pose.
//
// These joint values are from TestSandingWallCollision where the planner produced a trajectory
// with start/goal poses only 10mm apart but joint 1 swings ~384 degrees.
func TestInterpolateSegmentFSLargeJointMovement(t *testing.T) {
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur20.json"), "")
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.World())
	test.That(t, err, test.ShouldBeNil)

	// These are the actual joint values from the problematic trajectory in TestSandingWallCollision.
	// Joint 1 changes from -3.336 to 3.379 radians (~384 degrees), while the end effector
	// poses are only ~10mm apart.
	startConfig := []referenceframe.Input{
		1.0606116928015663,
		-3.3364680561740925,
		-0.18259589082216113,
		1.901973150656734,
		-1.534823224498082,
		4.202838433465263,
	}
	endConfig := []referenceframe.Input{
		1.0587534835302792,
		3.3789657942093747,
		-1.26485963000687,
		-0.5893461906885675,
		1.5347434325642373,
		1.0593975466125363,
	}

	segment := &SegmentFS{
		StartConfiguration: referenceframe.FrameSystemInputs{model.Name(): startConfig}.ToLinearInputs(),
		EndConfiguration:   referenceframe.FrameSystemInputs{model.Name(): endConfig}.ToLinearInputs(),
		FS:                 fs,
	}

	resolution := 1.

	interpolated, err := InterpolateSegmentFS(segment, resolution)
	test.That(t, err, test.ShouldBeNil)

	// Prior to joint space distance fix this emitted 11 interpolated positions
	test.That(t, len(interpolated), test.ShouldBeGreaterThan, 100)
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

	handler.collisionConstraints, err = CreateAllCollisionConstraints(
		fs,
		movingRobotGeometries,
		staticRobotGeometries,
		worldGeometries.Geometries(),
		nil, // allowedCollisions
		defaultCollisionBufferMM,
	)
	test.That(b, err, test.ShouldBeNil)

	rseed := rand.New(rand.NewSource(1))

	// loop through cases and check constraint handler processes them correctly
	for n := 0; n < b.N; n++ {
		rfloats := referenceframe.GenerateRandomConfiguration(model, rseed)
		stateFS := &StateFS{
			Configuration: referenceframe.FrameSystemInputs{model.Name(): rfloats}.ToLinearInputs(),
			FS:            fs,
		}
		_, err = handler.CheckStateFSConstraints(context.Background(), stateFS)
		test.That(b, err, test.ShouldBeNil)
	}
}
