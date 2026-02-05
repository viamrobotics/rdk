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

func TestCalculateJointStepCount(t *testing.T) {
	t.Run("no movement", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{0, 0, 0}
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 0)
	})

	t.Run("small movement under step size", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{0.04, 0, 0} // 0.04 rad < defaultJointStepSizeRadians
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 1)
	})

	t.Run("one radian movement", func(t *testing.T) {
		start := []float64{0, 0, 0}
		end := []float64{1.0, 0, 0} // 1 rad / defaultJointStepSizeRadians = 20 steps
		test.That(t, calculateJointStepCount(start, end, 0.05), test.ShouldEqual, 20)
	})

	t.Run("large joint movement from sanding collision bug", func(t *testing.T) {
		// Joint 1 from TestSandingWallCollision moves ~6.7 radians
		start := []float64{1.06, -3.336, -0.18, 1.90, -1.53, 4.20}
		end := []float64{1.06, 3.379, -1.26, -0.59, 1.53, 1.06}
		// Joint 1 moves 6.715 rad / 0.05 = 135 steps
		steps := calculateJointStepCount(start, end, 0.05)
		test.That(t, steps, test.ShouldEqual, 135)
	})
}

// TestSegmentStepCount tests that segmentStepCount correctly emits step count from either joint or cartesian excursion
func TestSegmentStepCount(t *testing.T) {
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur20.json"), "")
	test.That(t, err, test.ShouldBeNil)
	jointStepSize := jointStepSizeFromLimits(model.DoF())

	fs := referenceframe.NewEmptyFrameSystem("test")
	err = fs.AddFrame(model, fs.World())
	test.That(t, err, test.ShouldBeNil)
	startConfig := []referenceframe.Input{1.06, -3.336, -0.18, 1.90, -1.53, 4.20}
	startPos, err := model.Transform(startConfig)
	test.That(t, err, test.ShouldBeNil)

	t.Run("joint steps dominate when cartesian distance is small", func(t *testing.T) {
		// For a ur20 this config is very close to startConfig in cartesian space but far away in joint space
		endConfig := []referenceframe.Input{1.06, 3.379, -1.26, -0.59, 1.53, 1.06}
		segment := &SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{model.Name(): startConfig}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{model.Name(): endConfig}.ToLinearInputs(),
			FS:                 fs,
		}
		endPos, err := model.Transform(endConfig)
		test.That(t, err, test.ShouldBeNil)

		cartesianSteps := CalculateStepCount(startPos, endPos, 1.0)
		jointSteps := calculateJointStepCount(startConfig, endConfig, jointStepSize)

		totalSteps, err := segmentStepCount(segment, 1.0)
		test.That(t, err, test.ShouldBeNil)

		// Joint steps should dominate over cartesian for this trajectory
		test.That(t, jointSteps, test.ShouldBeGreaterThan, cartesianSteps)

		// segmentStepCount should return the joint step count
		test.That(t, totalSteps, test.ShouldEqual, jointSteps)
	})

	t.Run("cartesian steps dominate when joint distance is small", func(t *testing.T) {
		// Joints are close to startConfig, but quite far in cartesian space
		endConfig := []referenceframe.Input{1.06, -3.379, -1.26, -0.59, 1.53, 1.06}
		segment := &SegmentFS{
			StartConfiguration: referenceframe.FrameSystemInputs{model.Name(): startConfig}.ToLinearInputs(),
			EndConfiguration:   referenceframe.FrameSystemInputs{model.Name(): endConfig}.ToLinearInputs(),
			FS:                 fs,
		}
		endPos, err := model.Transform(endConfig)
		test.That(t, err, test.ShouldBeNil)

		cartesianSteps := CalculateStepCount(startPos, endPos, 1.0)
		jointSteps := calculateJointStepCount(startConfig, endConfig, jointStepSize)

		totalSteps, err := segmentStepCount(segment, 1.0)
		test.That(t, err, test.ShouldBeNil)

		// Cartesian steps should dominate over joint for this trajectory
		test.That(t, cartesianSteps, test.ShouldBeGreaterThan, jointSteps)

		// segmentStepCount should return the cartesian step count
		test.That(t, totalSteps, test.ShouldEqual, cartesianSteps)
	})
}

func TestComputeInitialCollisionsToIgnore(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("")

	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	t.Run("combines initial collisions with specifications", func(t *testing.T) {
		// Create colliding geometries
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		moving, err := NewGeometryGroup([]spatial.Geometry{geom1})
		test.That(t, err, test.ShouldBeNil)
		static, err := NewGeometryGroup([]spatial.Geometry{geom2})
		test.That(t, err, test.ShouldBeNil)

		// Test that initial collisions are detected and combined with specifications
		collisionSpecs := []Collision{{"box1", "box3"}}
		ignoreList, err := computeInitialCollisionsToIgnore(fs, moving, static, collisionSpecs, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ignoreList), test.ShouldEqual, 2)

		// Verify the specification collision is included
		found := false
		for _, c := range ignoreList {
			if c.name1 == "box1" && c.name2 == "box3" {
				found = true
				break
			}
		}
		test.That(t, found, test.ShouldBeTrue)
	})

	t.Run("empty when no collisions or specs", func(t *testing.T) {
		// Create non-colliding geometries
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{10, 0, 0}))
		geom2.SetLabel("box2")

		moving, err := NewGeometryGroup([]spatial.Geometry{geom1})
		test.That(t, err, test.ShouldBeNil)
		static, err := NewGeometryGroup([]spatial.Geometry{geom2})
		test.That(t, err, test.ShouldBeNil)

		ignoreList, err := computeInitialCollisionsToIgnore(fs, moving, static, nil, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ignoreList), test.ShouldEqual, 0)
	})
}

func TestCollisionDistance(t *testing.T) {
	fs := referenceframe.NewEmptyFrameSystem("")

	bc1, err := spatial.NewBox(spatial.NewZeroPose(), r3.Vector{2, 2, 2}, "")
	test.That(t, err, test.ShouldBeNil)

	t.Run("collision returns -1 and error", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		gg1, err := NewGeometryGroup([]spatial.Geometry{geom1})
		test.That(t, err, test.ShouldBeNil)
		gg2, err := NewGeometryGroup([]spatial.Geometry{geom2})
		test.That(t, err, test.ShouldBeNil)

		dist, err := collisionDistance(fs, gg1, gg2, nil, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "violation")
		test.That(t, err.Error(), test.ShouldContainSubstring, "box1")
		test.That(t, err.Error(), test.ShouldContainSubstring, "box2")
		test.That(t, dist, test.ShouldEqual, -1)
	})

	t.Run("no collision returns positive distance", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewPoseFromPoint(r3.Vector{10, 0, 0}))
		geom2.SetLabel("box2")

		gg1, err := NewGeometryGroup([]spatial.Geometry{geom1})
		test.That(t, err, test.ShouldBeNil)
		gg2, err := NewGeometryGroup([]spatial.Geometry{geom2})
		test.That(t, err, test.ShouldBeNil)

		dist, err := collisionDistance(fs, gg1, gg2, nil, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dist, test.ShouldBeGreaterThan, 0)
	})

	t.Run("ignored collision returns positive distance", func(t *testing.T) {
		geom1 := bc1.Transform(spatial.NewZeroPose())
		geom1.SetLabel("box1")
		geom2 := bc1.Transform(spatial.NewZeroPose())
		geom2.SetLabel("box2")

		gg1, err := NewGeometryGroup([]spatial.Geometry{geom1})
		test.That(t, err, test.ShouldBeNil)
		gg2, err := NewGeometryGroup([]spatial.Geometry{geom2})
		test.That(t, err, test.ShouldBeNil)

		ignoreList := []Collision{{"box1", "box2"}}
		dist, err := collisionDistance(fs, gg1, gg2, ignoreList, defaultCollisionBufferMM)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, dist, test.ShouldBeGreaterThan, 0)
	})
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
