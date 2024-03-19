// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestPTGKinematicsNoGeom(t *testing.T) {
	logger := logging.NewTestLogger(t)

	name := resource.Name{API: resource.NewAPI("is", "a", "fakebase")}
	b := &fake.Base{
		Named:         name.AsNamed(),
		Geometry:      []spatialmath.Geometry{},
		WidthMeters:   0.2,
		TurningRadius: 0.3,
	}

	ctx := context.Background()

	kb, err := WrapWithKinematics(ctx, b, logger, nil, nil, NewKinematicBaseOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb, test.ShouldNotBeNil)
	ptgBase, ok := kb.(*ptgBaseKinematics)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, ptgBase, test.ShouldNotBeNil)

	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: 999, Y: 0, Z: 0}))

	fs := referenceframe.NewEmptyFrameSystem("test")
	f := kb.Kinematics()

	defaultBaseGeom, err := spatialmath.NewSphere(spatialmath.NewZeroPose(), 150., b.Name().Name)
	test.That(t, err, test.ShouldBeNil)
	t.Run("Kinematics", func(t *testing.T) {
		frame, err := tpspace.NewPTGFrameFromKinematicOptions(
			b.Name().ShortName(), logger, 0.3, 0, nil, NewKinematicBaseOptions().NoSkidSteer, b.TurningRadius == 0,
		)
		test.That(t, frame, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, f.Name(), test.ShouldEqual, b.Name().ShortName())
		test.That(t, f.DoF(), test.ShouldResemble, frame.DoF())

		gifs, err := f.Geometries(referenceframe.FloatsToInputs([]float64{0, 0, 0}))
		test.That(t, err, test.ShouldBeNil)

		test.That(t, gifs.Geometries(), test.ShouldResemble, []spatialmath.Geometry{defaultBaseGeom})
	})
	t.Run("Geometries", func(t *testing.T) {
		geoms, err := kb.Geometries(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(geoms), test.ShouldEqual, 1)
		test.That(t, geoms[0], test.ShouldResemble, defaultBaseGeom)
	})

	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(f, fs.World())
	inputMap := referenceframe.StartPositions(fs)

	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             logger,
		Goal:               dstPIF,
		Frame:              f,
		StartConfiguration: inputMap,
		FrameSystem:        fs,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)
	for i, inputMap := range plan.Trajectory() {
		inputs := inputMap[""]
		selectedPTG := ptgBase.ptgs[int(math.Round(inputs[ptgIndex].Value))]

		selectedTraj, err := selectedPTG.Trajectory(
			inputs[trajectoryIndexWithinPTG].Value,
			inputs[distanceAlongTrajectoryIndex].Value,
			stepDistResolution,
		)
		test.That(t, err, test.ShouldBeNil)
		arcSteps := ptgBase.trajectoryToArcSteps(selectedTraj)

		if i == 0 || i == len(plan.Trajectory())-1 {
			// First and last should be all-zero stop commands
			test.That(t, len(arcSteps), test.ShouldEqual, 1)
			test.That(t, arcSteps[0].timestepSeconds, test.ShouldEqual, 0)
			test.That(t, arcSteps[0].linVelMMps, test.ShouldResemble, r3.Vector{})
			test.That(t, arcSteps[0].angVelDegps, test.ShouldResemble, r3.Vector{})
		} else {
			test.That(t, len(arcSteps), test.ShouldBeGreaterThanOrEqualTo, 1)
		}
	}
}

func TestPTGKinematicsWithGeom(t *testing.T) {
	logger := logging.NewTestLogger(t)

	name := resource.Name{API: resource.NewAPI("is", "a", "fakebase")}

	baseGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	b := &fake.Base{
		Named:         name.AsNamed(),
		Geometry:      []spatialmath.Geometry{baseGeom},
		WidthMeters:   0.2,
		TurningRadius: 0.3,
	}

	ctx := context.Background()

	kbOpt := NewKinematicBaseOptions()
	kbOpt.AngularVelocityDegsPerSec = 0

	ms := inject.NewMovementSensor("movement_sensor")
	gpOrigin := geo.NewPoint(0, 0)
	ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return gpOrigin, 0, nil
	}
	ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}
	localizer := motion.NewMovementSensorLocalizer(ms, gpOrigin, spatialmath.NewZeroPose())
	kb, err := WrapWithKinematics(ctx, b, logger, localizer, nil, kbOpt)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb, test.ShouldNotBeNil)

	ptgBase, ok := kb.(*ptgBaseKinematics)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, ptgBase, test.ShouldNotBeNil)

	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: 2000, Y: 0, Z: 0}))

	fs := referenceframe.NewEmptyFrameSystem("test")
	f := kb.Kinematics()
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(f, fs.World())
	inputMap := referenceframe.StartPositions(fs)

	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{1000, 0, 0}), r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	geoms := []spatialmath.Geometry{obstacle}
	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:             logger,
		Goal:               dstPIF,
		Frame:              f,
		StartConfiguration: inputMap,
		FrameSystem:        fs,
		WorldState:         worldState,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)

	inputs, err := plan.Trajectory().GetFrameInputs(kb.Name().ShortName())
	test.That(t, err, test.ShouldBeNil)
	ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		newPose, err := kb.Kinematics().Transform(inputs[1])
		test.That(t, err, test.ShouldBeNil)
		newGeoPose := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(gpOrigin, 0), newPose)
		return newGeoPose.Location(), 0, nil
	}

	t.Run("Kinematics", func(t *testing.T) {
		kinematics := kb.Kinematics()
		f, err := tpspace.NewPTGFrameFromKinematicOptions(
			b.Name().ShortName(), logger, 0.3, 0, []spatialmath.Geometry{baseGeom}, kbOpt.NoSkidSteer, b.TurningRadius == 0,
		)
		test.That(t, f, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, kinematics.Name(), test.ShouldEqual, b.Name().ShortName())
		test.That(t, kinematics.DoF(), test.ShouldResemble, f.DoF())

		gifs, err := kinematics.Geometries(referenceframe.FloatsToInputs([]float64{0, 0, 0}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gifs.Geometries(), test.ShouldResemble, []spatialmath.Geometry{baseGeom})
	})

	t.Run("GoToInputs", func(t *testing.T) {
		waypoints, err := plan.Trajectory().GetFrameInputs(kb.Name().ShortName())
		test.That(t, err, test.ShouldBeNil)
		err = kb.GoToInputs(ctx, waypoints[1])
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("CurrentInputs", func(t *testing.T) {
		currentInputs, err := kb.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(currentInputs), test.ShouldEqual, 3)
		expectedInputs := referenceframe.FloatsToInputs([]float64{0, 0, 0})
		test.That(t, currentInputs, test.ShouldResemble, expectedInputs)
	})

	t.Run("ErrorState", func(t *testing.T) {
		errorState, err := kb.ErrorState(ctx, plan, 2)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, errorState, test.ShouldNotBeNil)
		test.That(t, spatialmath.PoseAlmostCoincidentEps(errorState, spatialmath.NewZeroPose(), 1e-5), test.ShouldBeTrue)
	})

	t.Run("CurrentPosition", func(t *testing.T) {
		currentPosition, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentPosition, test.ShouldNotBeNil)
		expectedPosition, err := kb.Kinematics().Transform(inputs[1])
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostCoincidentEps(currentPosition.Pose(), expectedPosition, 1e-5), test.ShouldBeTrue)
	})

	t.Run("Geometries", func(t *testing.T) {
		geoms, err := kb.Geometries(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(geoms), test.ShouldEqual, 1)
		test.That(t, geoms[0], test.ShouldResemble, baseGeom)
	})
}
