package armplanning_test

import (
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/motionplan/armplanning"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

func TestWriteAndReadRequestAndResponse(t *testing.T) {
	// Build a minimal frame system with a 6-DOF arm.
	model, err := referenceframe.ParseModelJSONFile(utils.ResolveFile("components/arm/fake/kinematics/xarm6.json"), "xarm6")
	test.That(t, err, test.ShouldBeNil)

	fs := referenceframe.NewEmptyFrameSystem("")
	err = fs.AddFrame(model, fs.World())
	test.That(t, err, test.ShouldBeNil)

	startInputs := referenceframe.FrameSystemInputs{"xarm6": make([]referenceframe.Input, 6)}
	goalPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 200, Y: 100, Z: 300})
	goalPoses := referenceframe.FrameSystemPoses{
		"xarm6": referenceframe.NewPoseInFrame(referenceframe.World, goalPose),
	}

	req := &armplanning.PlanRequest{
		FrameSystem: fs,
		StartState:  armplanning.NewPlanState(nil, startInputs),
		Goals:       []*armplanning.PlanState{armplanning.NewPlanState(goalPoses, nil)},
	}

	// Build a trivial plan response: two identical waypoints.
	traj := motionplan.Trajectory{
		referenceframe.FrameSystemInputs{"xarm6": {0.1, 0.2, 0.3, 0.4, 0.5, 0.6}},
		referenceframe.FrameSystemInputs{"xarm6": {0.7, 0.8, 0.9, 1.0, 1.1, 1.2}},
	}
	resp := motionplan.NewSimplePlan(nil, traj)

	fileName := filepath.Join(t.TempDir(), "plan.json")
	t.Run("WriteRequestAndResponseToFile", func(t *testing.T) {
		err = req.WriteRequestAndResponseToFile(fileName, resp)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("ReadRequestFromFile parses the request", func(t *testing.T) {
		got, err := armplanning.ReadRequestFromFile(fileName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, got, test.ShouldNotBeNil)
		test.That(t, got.FrameSystem, test.ShouldNotBeNil)
		test.That(t, got.StartState, test.ShouldNotBeNil)
		test.That(t, got.StartState.Configuration(), test.ShouldResemble, startInputs)
		test.That(t, len(got.Goals), test.ShouldEqual, 1)
	})

	t.Run("ReadRequestAndResponseFromFile parses both", func(t *testing.T) {
		gotReq, gotPlan, err := armplanning.ReadRequestAndResponseFromFile(fileName)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotReq, test.ShouldNotBeNil)
		test.That(t, gotReq.StartState.Configuration(), test.ShouldResemble, startInputs)
		test.That(t, gotPlan, test.ShouldNotBeNil)
		test.That(t, gotPlan.Trajectory(), test.ShouldResemble, traj)
	})

	t.Run("ReadRequestAndResponseFromFile with nil response returns nil plan", func(t *testing.T) {
		requestOnlyFile := filepath.Join(t.TempDir(), "request_only.json")
		err = req.WriteToFile(requestOnlyFile)
		test.That(t, err, test.ShouldBeNil)

		gotReq, gotPlan, err := armplanning.ReadRequestAndResponseFromFile(requestOnlyFile)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, gotReq, test.ShouldNotBeNil)
		test.That(t, gotPlan, test.ShouldBeNil)
	})
}
