package objectmanipulation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/testutils/inject"
)

func TestDoGrabFailures(t *testing.T) {
	cfgService := config.Service{Name: "objectmanipulation"}
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
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("fakeArm"), gripper.Named("fakeGripper")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, bool) {
		switch n.Name {
		case "fakeArm":
			return _arm, true
		case "fakeGripper":
			return _gripper, true
		default:
			return nil, false
		}
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
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/moving_arm.json")
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := objectmanipulation.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = svc.DoGrab(ctx, "pieceGripper", "pieceArm", "c", &r3.Vector{-20, -30, -40})
	test.That(t, err, test.ShouldBeNil)
}

func TestMultiplePieces(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/fake_tomato.json")
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := objectmanipulation.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = svc.DoGrab(ctx, "gr", "a", "c", &r3.Vector{-20, -30, -40})
	test.That(t, err, test.ShouldBeNil)

	// remove after this
	theArm, _ := myRobot.ArmByName("a")
	temp, _ := theArm.GetJointPositions(ctx)
	logger.Debugf("end arm position; %v", temp)
}
