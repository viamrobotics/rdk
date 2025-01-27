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
	"go.viam.com/rdk/motionplan/ik"
	"go.viam.com/rdk/motionplan/tpspace"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestPTGKinematicsNoGeom(t *testing.T) {
	logger := logging.NewTestLogger(t)

	name := resource.Name{API: resource.NewAPI("is", "a", "fakebase"), Name: "fakebase"}
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

		gifs, err := f.Geometries(referenceframe.FloatsToInputs([]float64{0, 0, 0, 0}))
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
	inputMap := referenceframe.NewZeroInputs(fs)

	startState := motionplan.NewPlanState(
		referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewZeroPoseInFrame(referenceframe.World)},
		inputMap,
	)
	goalState := motionplan.NewPlanState(referenceframe.FrameSystemPoses{f.Name(): dstPIF}, nil)

	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:      logger,
		Goals:       []*motionplan.PlanState{goalState},
		StartState:  startState,
		FrameSystem: fs,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)
	runningPose := spatialmath.NewZeroPose()
	for i, inputMap := range plan.Trajectory() {
		inputs := inputMap["fakebase"]
		arcSteps, err := ptgBase.trajectoryArcSteps(runningPose, inputs)
		test.That(t, err, test.ShouldBeNil)

		if i == 0 || i == len(plan.Trajectory())-1 {
			// First and last should be all-zero stop commands
			test.That(t, len(arcSteps), test.ShouldEqual, 1)
			test.That(t, arcSteps[0].durationSeconds, test.ShouldEqual, 0)
			test.That(t, arcSteps[0].linVelMMps, test.ShouldResemble, r3.Vector{})
			test.That(t, arcSteps[0].angVelDegps, test.ShouldResemble, r3.Vector{})
		} else {
			test.That(t, len(arcSteps), test.ShouldBeGreaterThanOrEqualTo, 1)
		}
		runningPose = spatialmath.Compose(runningPose, arcSteps[len(arcSteps)-1].subTraj[len(arcSteps[len(arcSteps)-1].subTraj)-1].Pose)
	}
}

func TestPTGKinematicsWithGeom(t *testing.T) {
	logger := logging.NewTestLogger(t)

	name := resource.Name{API: resource.NewAPI("is", "a", "fakebase"), Name: "fakebase"}

	baseGeom, err := spatialmath.NewBox(spatialmath.NewZeroPose(), r3.Vector{1, 1, 1}, "fakebase")
	test.That(t, err, test.ShouldBeNil)

	b := &fake.Base{
		Named:         name.AsNamed(),
		Geometry:      []spatialmath.Geometry{baseGeom},
		WidthMeters:   0.2,
		TurningRadius: 0.3,
	}

	ctx := context.Background()

	kbOpt := NewKinematicBaseOptions()
	kbOpt.AngularVelocityDegsPerSec = 20

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

	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: 6000, Y: 0, Z: 0}))

	fs := referenceframe.NewEmptyFrameSystem("test")
	f := kb.Kinematics()
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(f, fs.World())
	inputMap := referenceframe.NewZeroInputs(fs)

	obstacle, err := spatialmath.NewBox(spatialmath.NewPoseFromPoint(r3.Vector{2000, 0, 0}), r3.Vector{1, 1, 1}, "")
	test.That(t, err, test.ShouldBeNil)

	geoms := []spatialmath.Geometry{obstacle}
	worldState, err := referenceframe.NewWorldState(
		[]*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)},
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	startState := motionplan.NewPlanState(
		referenceframe.FrameSystemPoses{f.Name(): referenceframe.NewZeroPoseInFrame(referenceframe.World)},
		inputMap,
	)
	goalState := motionplan.NewPlanState(referenceframe.FrameSystemPoses{f.Name(): dstPIF}, nil)
	plan, err := motionplan.PlanMotion(ctx, &motionplan.PlanRequest{
		Logger:      logger,
		Goals:       []*motionplan.PlanState{goalState},
		StartState:  startState,
		FrameSystem: fs,
		WorldState:  worldState,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)

	allInputs := [][]referenceframe.Input{}

	// Spot check each individual trajectory
	runningPose := spatialmath.NewZeroPose()
	for i, inputMap := range plan.Trajectory() {
		inputs := inputMap["fakebase"]
		allInputs = append(allInputs, inputs)
		arcSteps, err := ptgBase.trajectoryArcSteps(runningPose, inputs)
		test.That(t, err, test.ShouldBeNil)

		if i == 0 || i == len(plan.Trajectory())-1 {
			// First and last should be all-zero stop commands
			test.That(t, len(arcSteps), test.ShouldEqual, 1)
			test.That(t, arcSteps[0].durationSeconds, test.ShouldEqual, 0)
			test.That(t, arcSteps[0].linVelMMps, test.ShouldResemble, r3.Vector{})
			test.That(t, arcSteps[0].angVelDegps, test.ShouldResemble, r3.Vector{})
		} else {
			test.That(t, len(arcSteps), test.ShouldBeGreaterThanOrEqualTo, 1)
		}
		runningPose = spatialmath.Compose(runningPose, arcSteps[len(arcSteps)-1].subTraj[len(arcSteps[len(arcSteps)-1].subTraj)-1].Pose)
	}

	// Now check the full set of arcs
	arcSteps, err := ptgBase.arcStepsFromInputs(allInputs, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)
	arcIdx := 1

	t.Run("CourseCorrectionPieces", func(t *testing.T) {
		currInputs := []referenceframe.Input{
			arcSteps[arcIdx].arcSegment.StartConfiguration[0],
			arcSteps[arcIdx].arcSegment.StartConfiguration[1],
			{0},
			{1},
		}
		ptgBase.inputLock.Lock()
		ptgBase.currentState.currentIdx = arcIdx
		ptgBase.currentState.currentExecutingSteps = arcSteps
		ptgBase.currentState.currentInputs = currInputs
		ptgBase.inputLock.Unlock()
		// Mock up being off course and try to correct
		skewPose := spatialmath.NewPose(r3.Vector{5, -300, 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -4})
		newPose, err := kb.Kinematics().Transform(currInputs)
		test.That(t, err, test.ShouldBeNil)

		ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			newGeoPose := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(gpOrigin, 0), spatialmath.Compose(newPose, skewPose))
			return newGeoPose.Location(), 0, nil
		}

		goals := ptgBase.makeCourseCorrectionGoals(
			goalsToAttempt,
			arcIdx,
			skewPose,
			arcSteps,
			currInputs,
		)
		test.That(t, goals, test.ShouldNotBeNil)
		solution, err := ptgBase.getCorrectionSolution(ctx, goals)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, solution, test.ShouldNotBeNil)

		t.Run("ErrorState", func(t *testing.T) {
			executionState, err := kb.ExecutionState(ctx)
			test.That(t, err, test.ShouldBeNil)
			errorState, err := motionplan.CalculateFrameErrorState(executionState, kb.Kinematics(), kb.LocalizationFrame())
			test.That(t, err, test.ShouldBeNil)

			// Error State should be computed based on current inputs, current executing steps, and the localizer's position function
			currentPosition, err := kb.CurrentPosition(ctx)
			test.That(t, err, test.ShouldBeNil)

			arcStartPosition := arcSteps[arcIdx].arcSegment.StartPosition
			onArcPosition, err := kb.Kinematics().Transform(ptgBase.currentState.currentInputs)
			test.That(t, err, test.ShouldBeNil)
			arcPose := spatialmath.Compose(arcStartPosition, onArcPosition)

			test.That(
				t,
				spatialmath.PoseAlmostCoincidentEps(errorState, spatialmath.PoseBetween(arcPose, currentPosition.Pose()), 1e-5),
				test.ShouldBeTrue,
			)
			test.That(
				t,
				spatialmath.PoseAlmostCoincidentEps(errorState, spatialmath.PoseBetween(arcPose, skewPose), 5),
				test.ShouldBeTrue,
			)
		})

		t.Run("RunCorrection", func(t *testing.T) {
			newArcSteps, err := ptgBase.courseCorrect(ctx, currInputs, arcSteps, arcIdx)
			test.That(t, err, test.ShouldBeNil)
			arcIdx++
			newInputs := []referenceframe.Input{
				arcSteps[arcIdx].arcSegment.StartConfiguration[0],
				arcSteps[arcIdx].arcSegment.StartConfiguration[1],
				{0},
				{0},
			}
			ptgBase.inputLock.Lock()
			ptgBase.currentState.currentIdx = arcIdx
			ptgBase.currentState.currentExecutingSteps = newArcSteps
			ptgBase.currentState.currentInputs = newInputs
			ptgBase.inputLock.Unlock()
			// After course correction, error state should always be zero
			executionState, err := kb.ExecutionState(ctx)
			test.That(t, err, test.ShouldBeNil)
			errorState, err := motionplan.CalculateFrameErrorState(executionState, kb.Kinematics(), kb.LocalizationFrame())
			test.That(t, err, test.ShouldBeNil)
			test.That(t, spatialmath.PoseAlmostEqualEps(errorState, spatialmath.NewZeroPose(), 1e-5), test.ShouldBeTrue)
		})
	})

	t.Run("EasyGoal", func(t *testing.T) {
		goal := courseCorrectionGoal{
			Goal: spatialmath.NewPose(r3.Vector{X: -0.8564, Y: 234.}, &spatialmath.OrientationVectorDegrees{OZ: 1., Theta: 4.4}),
		}
		solution, err := ptgBase.getCorrectionSolution(ctx, []courseCorrectionGoal{goal})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, solution.Solution, test.ShouldNotBeNil) // Irrelevant what this is as long as filled in
	})

	t.Run("Kinematics", func(t *testing.T) {
		kinematics := kb.Kinematics()
		f, err := tpspace.NewPTGFrameFromKinematicOptions(
			b.Name().ShortName(), logger, 0.3, 0, []spatialmath.Geometry{baseGeom}, kbOpt.NoSkidSteer, b.TurningRadius == 0,
		)
		test.That(t, f, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, kinematics.Name(), test.ShouldEqual, b.Name().ShortName())
		test.That(t, kinematics.DoF(), test.ShouldResemble, f.DoF())

		gifs, err := kinematics.Geometries(referenceframe.FloatsToInputs([]float64{0, 0, 0, 0}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gifs.Geometries(), test.ShouldResemble, []spatialmath.Geometry{baseGeom})
	})

	t.Run("GoToInputs", func(t *testing.T) {
		// The transform of current inputs is the remaining step, i.e. where the base will go when the current inputs are done executing.
		// To mock this up correctly, we need to alter the inputs here to simulate the base moving without a localizer.
		// Due to limitations of this mockup, this must use a PTG which will produce only one arc step. Full motion testing is provided by
		// the motion service.
		ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			ptgBase.inputLock.RLock()
			currInputs := ptgBase.currentState.currentInputs
			ptgBase.inputLock.RUnlock()
			currInputs[2].Value = 0
			newPose, err := kb.Kinematics().Transform(currInputs)

			test.That(t, err, test.ShouldBeNil)
			newGeoPose := spatialmath.PoseToGeoPose(spatialmath.NewGeoPose(gpOrigin, 0), newPose)
			return newGeoPose.Location(), 0, nil
		}
		ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			ptgBase.inputLock.RLock()
			currInputs := ptgBase.currentState.currentInputs
			ptgBase.inputLock.RUnlock()
			currInputs[2].Value = 0
			newPose, err := kb.Kinematics().Transform(currInputs)
			test.That(t, err, test.ShouldBeNil)
			headingRightHanded := newPose.Orientation().OrientationVectorDegrees().Theta
			return math.Abs(headingRightHanded) - 360, nil
		}

		waypoints, err := plan.Trajectory().GetFrameInputs(kb.Name().ShortName())
		test.That(t, err, test.ShouldBeNil)
		// Start by resetting current inputs to 0
		err = kb.GoToInputs(ctx, waypoints[0])
		test.That(t, err, test.ShouldBeNil)
		newInputs := []referenceframe.Input{
			{float64(ptgBase.courseCorrectionIdx)},
			{math.Pi / 2.},
			{0},
			{1100},
		}
		err = kb.GoToInputs(ctx, newInputs)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("CurrentInputs", func(t *testing.T) {
		currentInputs, err := kb.CurrentInputs(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(currentInputs), test.ShouldEqual, 4)
		expectedInputs := referenceframe.FloatsToInputs([]float64{0, 0, 0, 0})
		test.That(t, currentInputs, test.ShouldResemble, expectedInputs)
	})

	t.Run("CurrentPosition", func(t *testing.T) {
		currentPosition, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentPosition, test.ShouldNotBeNil)
		expectedPosition, err := kb.Kinematics().Transform(ptgBase.currentState.currentInputs)
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

func TestPTGKinematicsSimpleInputs(t *testing.T) {
	logger := logging.NewTestLogger(t)

	name := resource.Name{API: resource.NewAPI("is", "a", "fakebase"), Name: "fakebase"}
	b := &fake.Base{
		Named:         name.AsNamed(),
		Geometry:      []spatialmath.Geometry{},
		WidthMeters:   0.2,
		TurningRadius: 0,
	}

	ctx := context.Background()
	kbo := NewKinematicBaseOptions()
	kbo.NoSkidSteer = true
	kbo.UpdateStepSeconds = 0.01

	kb, err := WrapWithKinematics(ctx, b, logger, nil, nil, kbo)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb, test.ShouldNotBeNil)
	ptgBase, ok := kb.(*ptgBaseKinematics)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, ptgBase, test.ShouldNotBeNil)

	inputs := []referenceframe.Input{{0}, {1.9}, {1300}, {200}}
	err = ptgBase.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)

	inputs = []referenceframe.Input{{0}, {1.9}, {1300}, {0}}
	err = ptgBase.GoToInputs(ctx, inputs)
	test.That(t, err, test.ShouldBeNil)
}

func TestCopyArcStep(t *testing.T) {
	step := &arcStep{
		linVelMMps:      r3.Vector{1, 2, 3},
		angVelDegps:     r3.Vector{4, 5, 6},
		durationSeconds: 3.14,
		arcSegment: ik.Segment{
			StartPosition:      spatialmath.NewPoseFromPoint(r3.Vector{1, 2, 3}),
			EndPosition:        spatialmath.NewPoseFromPoint(r3.Vector{4, 5, 6}),
			StartConfiguration: []referenceframe.Input{{1}, {2}, {3}},
			EndConfiguration:   []referenceframe.Input{{4}, {5}, {6}},
			Frame:              referenceframe.NewZeroStaticFrame("test"),
		},
		subTraj: []*tpspace.TrajNode{
			{
				Pose:   spatialmath.NewPoseFromPoint(r3.Vector{7, 8, 9}),
				Dist:   2.72,
				Alpha:  1.61,
				LinVel: 0.11,
				AngVel: 0.22,
			},
		},
	}

	copiedStep := copyArcStep(*step)
	test.That(t, &copiedStep, test.ShouldResemble, step)
}
