package armplanning

import (
	"context"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	frame "go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

// testPlan is a minimal motionplan.Plan implementation used in tests.
type testPlan struct {
	trajectory motionplan.Trajectory
}

func (p *testPlan) Trajectory() motionplan.Trajectory { return p.trajectory }
func (p *testPlan) Path() motionplan.Path             { return nil }

func TestCheckPlan(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	ur20, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur20.json"), "")
	test.That(t, err, test.ShouldBeNil)

	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(ur20, fs.World())
	test.That(t, err, test.ShouldBeNil)

	startInputs := []frame.Input{
		0.7853981633974483,
		-0.7853981633974483,
		1.5707963267948966,
		-0.7853981633974483,
		0.7853981633974483,
		0,
	}

	bigWall, err := spatialmath.NewBox(
		spatialmath.NewPose(
			r3.Vector{X: 499.80892449234604, Y: 0, Z: 0},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
		),
		r3.Vector{X: 100, Y: 6774.340100002068, Z: 4708.262746117678},
		"bigWall",
	)
	test.That(t, err, test.ShouldBeNil)

	littleWall1, err := spatialmath.NewBox(
		spatialmath.NewPose(
			r3.Vector{X: -489.0617925579456, Y: 0, Z: 0},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
		),
		r3.Vector{X: 693.3530661392058, Y: 100, Z: 725.3808831665151},
		"littleWall1",
	)
	test.That(t, err, test.ShouldBeNil)

	littleWall2, err := spatialmath.NewBox(
		spatialmath.NewPose(
			r3.Vector{X: -812.5564475789858, Y: 0, Z: 295.11694017940315},
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0},
		),
		r3.Vector{X: 369.641316443868, Y: 100, Z: 596.3239775519398},
		"littleWall2",
	)
	test.That(t, err, test.ShouldBeNil)

	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{
			frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{bigWall, littleWall1, littleWall2}),
		},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	goalPose := spatialmath.NewPose(
		r3.Vector{
			X: -1091.7784630090632,
			Y: 653.2215369909372,
			Z: 171.2573338461849,
		},
		&spatialmath.OrientationVectorDegrees{
			OX:    -0.9999999999999999,
			OY:    -5.551115123125783e-17,
			OZ:    -8.495620873461007e-11,
			Theta: 89.9999999833808,
		},
	)

	planRequest := &PlanRequest{
		FrameSystem: fs,
		Goals: []*PlanState{
			{poses: frame.FrameSystemPoses{ur20.Name(): frame.NewPoseInFrame(frame.World, goalPose)}},
		},
		StartState:     &PlanState{structuredConfiguration: frame.FrameSystemInputs{ur20.Name(): startInputs}},
		WorldState:     worldState,
		PlannerOptions: NewBasicPlannerOptions(),
	}

	plan, _, err := PlanMotion(ctx, logger, planRequest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)

	err = CheckPlan(ctx, logger, worldState, fs, plan)
	test.That(t, err, test.ShouldBeNil)

	err = CheckPlanFromRequest(ctx, logger, planRequest, plan)
	test.That(t, err, test.ShouldBeNil)
}

// TestCheckPlanWithAllowedCollisions verifies that collisions present at the start configuration
// are allowed throughout the plan and do not cause CheckPlan to fail.
func TestCheckPlanWithAllowedCollisions(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	ur5, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(ur5, fs.World())
	test.That(t, err, test.ShouldBeNil)

	startInputs := []frame.Input{0, -1.5708, 1.5708, 0, 0, 0}

	// Find where the forearm link is so we can place an obstacle on top of it.
	startLinearInputs := frame.FrameSystemInputs{ur5.Name(): startInputs}.ToLinearInputs()
	fsGeoms, err := frame.FrameSystemGeometriesLinearInputs(fs, startLinearInputs)
	test.That(t, err, test.ShouldBeNil)

	var forearmCenter r3.Vector
	for _, geomsInFrame := range fsGeoms {
		for _, geom := range geomsInFrame.Geometries() {
			if geom.Label() == "UR5e:forearm_link" {
				forearmCenter = geom.Pose().Point()
				break
			}
		}
	}

	// Create an obstacle overlapping the forearm link at the start configuration.
	obstacle, err := spatialmath.NewBox(
		spatialmath.NewPose(forearmCenter, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0}),
		r3.Vector{X: 300, Y: 300, Z: 300},
		"obstacle",
	)
	test.That(t, err, test.ShouldBeNil)

	worldState, err := frame.NewWorldState(
		[]*frame.GeometriesInFrame{
			frame.NewGeometriesInFrame(frame.World, []spatialmath.Geometry{obstacle}),
		},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	// A trajectory that doesn't move — the start-state collision should be allowed throughout.
	plan := &testPlan{
		trajectory: motionplan.Trajectory{
			{ur5.Name(): startInputs},
			{ur5.Name(): startInputs},
		},
	}

	err = CheckPlan(ctx, logger, worldState, fs, plan)
	test.That(t, err, test.ShouldBeNil)
}

// TestCheckPlanNilWorldState verifies that a nil WorldState is handled gracefully (treated as
// having no obstacles) rather than panicking.
func TestCheckPlanNilWorldState(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	ur5, err := frame.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/ur5e.json"), "")
	test.That(t, err, test.ShouldBeNil)

	fs := frame.NewEmptyFrameSystem("test")
	err = fs.AddFrame(ur5, fs.World())
	test.That(t, err, test.ShouldBeNil)

	plan := &testPlan{
		trajectory: motionplan.Trajectory{
			{ur5.Name(): []frame.Input{0, 0, 0, 0, 0, 0}},
			{ur5.Name(): []frame.Input{0.5, 0, 0, 0, 0, 0}},
		},
	}

	req := &PlanRequest{FrameSystem: fs, WorldState: nil}
	err = CheckPlanFromRequest(ctx, logger, req, plan)
	test.That(t, err, test.ShouldBeNil)
}
