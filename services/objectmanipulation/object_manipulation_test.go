package objectmanipulation_test

import (
	"context"
	"errors"
	"testing"

	"go.viam.com/test"

	"go.viam.com/core/component/arm"
	"go.viam.com/core/config"
	"go.viam.com/core/gripper"
	pb "go.viam.com/core/proto/api/v1"
	"go.viam.com/core/referenceframe"
	"go.viam.com/core/services/objectmanipulation"
	"go.viam.com/core/spatialmath"
	"go.viam.com/core/testutils/inject"
	"go.viam.com/core/utils"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
)

func TestDoGrabFailures(t *testing.T) {
	cfgService := config.Service{Name: "objectmanipulation", Type: objectmanipulation.Type}
	logger := golog.NewTestLogger(t)

	var r *inject.Robot
	var _gripper *inject.Gripper
	var _arm *inject.Arm

	// fails on not finding gripper

	r = &inject.Robot{}
	r.GripperByNameFunc = func(string) (gripper.Gripper, bool) {
		return nil, false
	}
	mgs, err := objectmanipulation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = mgs.DoGrab(context.Background(), "fakeGripper", "fakeArm", "fakeCamera", &r3.Vector{10.0, 10.0, 10.0})
	test.That(t, err, test.ShouldNotBeNil)

	// fails when gripper fails to open
	r = &inject.Robot{}
	_arm = &inject.Arm{}
	r.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		return _arm, true
	}
	_gripper = &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return errors.New("failure to open")
	}
	r.GripperByNameFunc = func(string) (gripper.Gripper, bool) {
		return _gripper, true
	}
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	mgs, _ = objectmanipulation.New(context.Background(), r, cfgService, logger)

	_, err = mgs.DoGrab(context.Background(), "fakeGripper", "fakeArm", "fakeCamera", &r3.Vector{10.0, 10.0, 10.0})
	test.That(t, err, test.ShouldNotBeNil)

	r = &inject.Robot{}
	_arm = &inject.Arm{}
	r.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		return _arm, true
	}
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	_gripper = &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return nil
	}
	// can't move gripper with respect to gripper
	_, err = mgs.DoGrab(context.Background(), "fakeGripperName", "fakeArm", "fakeGripperName", &r3.Vector{0, 0, 200})
	test.That(t, err, test.ShouldBeError, "cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
}

func TestDoGrab(t *testing.T) {
	logger := golog.NewTestLogger(t)
	cfgService := config.Service{Name: "objectmanipulation", Type: objectmanipulation.Type}

	var r *inject.Robot
	var _gripper *inject.Gripper
	var _arm *inject.Arm

	r = &inject.Robot{}

	_gripper = &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return nil
	}
	_gripper.GrabFunc = func(ctx context.Context) (bool, error) {
		return false, nil
	}

	_arm = &inject.Arm{}
	_arm.CurrentJointPositionsFunc = func(ctx context.Context) (*pb.JointPositions, error) {
		return &pb.JointPositions{
			Degrees: []float64{0, 0, 0, 0, 0, 0},
		}, nil
	}
	_arm.MoveToJointPositionsFunc = func(ctx context.Context, pos *pb.JointPositions) error {
		return nil
	}

	r.ArmByNameFunc = func(name string) (arm.Arm, bool) {
		return _arm, true
	}
	r.GripperByNameFunc = func(name string) (gripper.Gripper, bool) {
		return _gripper, true
	}
	r.LoggerFunc = func() golog.Logger {
		return logger
	}

	gripperName := "fakeGripper"
	armName := "fakeArm"

	fs := referenceframe.NewEmptySimpleFrameSystem("fakeGripper")

	pose := spatialmath.NewPoseFromPoint(r3.Vector{X: 5, Y: 0, Z: 5})
	gripperFrame, err := referenceframe.NewStaticFrame("fakeGripper", pose)
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gripperFrame, fs.World())
	r.FrameSystemFunc = func(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
		return fs, nil
	}
	mgs, err := objectmanipulation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)
	grabbed, err := mgs.DoGrab(context.Background(), gripperName, armName, "world", &r3.Vector{X: 500.0, Y: 0.0, Z: 500.0})
	test.That(t, grabbed, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeError, errors.New("solver frame has no degrees of freedom, cannot perform inverse kinematics"))

	fs = referenceframe.NewEmptySimpleFrameSystem("fakeGripper")
	gripperFrame, err = referenceframe.ParseJSONFile(utils.ResolveFile("robots/fake/arm_model.json"), "fakeGripper")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gripperFrame, fs.World())
	r.FrameSystemFunc = func(ctx context.Context, name, prefix string) (referenceframe.FrameSystem, error) {
		return fs, nil
	}

	mgs, err = objectmanipulation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	grabbed, err = mgs.DoGrab(context.Background(), gripperName, armName, "world", &r3.Vector{X: 500.0, Y: 0.0, Z: 500.0})
	test.That(t, grabbed, test.ShouldBeFalse)
	test.That(t, err, test.ShouldBeNil)
}
