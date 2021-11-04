package robotimpl_test

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/config"
	robotimpl "go.viam.com/core/robot/impl"
	"go.viam.com/core/spatialmath"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func TestGripperThatDoesntMove(t *testing.T) {
	cfg, err := config.Read("data/fake.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	test.That(t, len(r.CameraNames()), test.ShouldEqual, 1)
	cameraName := r.CameraNames()[0]

	err = robotimpl.MoveGripper(context.Background(), r, spatialmath.NewPoseFromPoint(r3.Vector{500, 0, 0}), cameraName)
	test.That(t, err, test.ShouldBeError, "cannot set upper or lower bounds for nlopt, slice is empty")
}

func TestMoveInGripperFrame(t *testing.T) {
	cfg, err := config.Read("data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	gripperName := r.GripperNames()[0]

	// can't move gripper with respect to gripper
	goal := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 200})
	err = robotimpl.MoveGripper(context.Background(), r, goal, gripperName)
	test.That(t, err, test.ShouldBeError, "cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
}

func TestMovingGripper(t *testing.T) {
	cfg, err := config.Read("data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	// don't move at all
	goal := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 200}) // gripper has implicit 200 z offset
	err = robotimpl.MoveGripper(context.Background(), r, goal, r.ArmNames()[0])
	test.That(t, err, test.ShouldBeNil)

	// move to a different location
	goal = spatialmath.NewPoseFromPoint(r3.Vector{500, 0, 500})
	err = robotimpl.MoveGripper(context.Background(), r, goal, "world")
	test.That(t, err, test.ShouldBeNil)
}
