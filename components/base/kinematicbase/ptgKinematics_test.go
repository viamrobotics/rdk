// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"math"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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
	kb, err := WrapWithKinematics(ctx, b, logger, nil, nil, kbOpt)
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
}
