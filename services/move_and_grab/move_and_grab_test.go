package moveandgrab_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	robotimpl "go.viam.com/core/robot/impl"
	moveandgrab "go.viam.com/core/services/move_and_grab"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/testutils/inject"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func TestGripperThatDoesntMove(t *testing.T) {
	cfg, err := config.Read("../../robot/impl/data/fake.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	test.That(t, len(r.CameraNames()), test.ShouldEqual, 1)
	cameraName := r.CameraNames()[0]

	cfgService := config.Service{Name: "moveandgrab", Type: moveandgrab.Type}
	mgs, err := moveandgrab.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	err = mgs.MoveGripper(context.Background(), spatialmath.NewPoseFromPoint(r3.Vector{500, 0, 0}), cameraName)
	test.That(t, err, test.ShouldBeError, "cannot set upper or lower bounds for nlopt, slice is empty")
}

func TestMoveInGripperFrame(t *testing.T) {
	cfg, err := config.Read("../../robot/impl/data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	cfgService := config.Service{Name: "moveandgrab", Type: moveandgrab.Type}
	mgs, err := moveandgrab.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	gripperName := r.GripperNames()[0]

	// can't move gripper with respect to gripper
	goal := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 200})
	err = mgs.MoveGripper(context.Background(), goal, gripperName)
	test.That(t, err, test.ShouldBeError, "cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
}

func TestMovingGripper(t *testing.T) {
	cfg, err := config.Read("../../robot/impl/data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	cfgService := config.Service{Name: "moveandgrab", Type: moveandgrab.Type}
	mgs, err := moveandgrab.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	// don't move at all
	goal := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 200}) // gripper has implicit 200 z offset
	err = mgs.MoveGripper(context.Background(), goal, r.ArmNames()[0])
	test.That(t, err, test.ShouldBeNil)

	// move to a different location
	goal = spatialmath.NewPoseFromPoint(r3.Vector{500, 0, 500})
	err = mgs.MoveGripper(context.Background(), goal, "world")
	test.That(t, err, test.ShouldBeNil)
}

func TestDoGrabFailures(t *testing.T) {
	// fails on multiple grippers
	r := &inject.Robot{}
	r.GripperNamesFunc = func() []string {
		return []string{"fake1", "fake2"}
	}
	logger := golog.NewTestLogger(t)

	cfgService := config.Service{Name: "moveandgrab", Type: moveandgrab.Type}
	mgs, err := moveandgrab.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = mgs.DoGrab(context.Background(), "fakeCamera", 10.0, 10.0, 10.0)
	test.That(t, err, test.ShouldNotBeNil)

	// fails on not finding gripper
	r = &inject.Robot{}
	r.GripperNamesFunc = func() []string {
		return []string{"fake1"}
	}
	r.GripperByNameFunc = func(string) (gripper.Gripper, bool) {
		return nil, false
	}
	mgs, _ = moveandgrab.New(context.Background(), r, cfgService, logger)

	_, err = mgs.DoGrab(context.Background(), "fakeCamera", 10.0, 10.0, 10.0)
	test.That(t, err, test.ShouldNotBeNil)

	// fails when gripper fails to open
	r = &inject.Robot{}
	r.GripperNamesFunc = func() []string {
		return []string{"fake1"}
	}
	_gripper := &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return errors.New("failure to open")
	}
	r.GripperByNameFunc = func(string) (gripper.Gripper, bool) {
		return _gripper, true
	}
	mgs, _ = moveandgrab.New(context.Background(), r, cfgService, logger)

	_, err = mgs.DoGrab(context.Background(), "fakeCamera", 10.0, 10.0, 10.0)
	test.That(t, err, test.ShouldNotBeNil)
}

func TestDoGrab(t *testing.T) {
	cfg, err := config.Read("../../robot/impl/data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)
	logger := golog.NewTestLogger(t)

	r, err := robotimpl.New(context.Background(), cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r.Close(), test.ShouldBeNil)

	cfgService := config.Service{Name: "moveandgrab", Type: moveandgrab.Type}
	mgs, err := moveandgrab.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	grabbed, err := mgs.DoGrab(context.Background(), "world", 500.0, 0.0, 500.0)
	test.That(t, grabbed, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)
}
