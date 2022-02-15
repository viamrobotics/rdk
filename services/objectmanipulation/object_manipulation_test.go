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
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)

	var r *inject.Robot
	var _gripper *inject.Gripper
	var _arm *inject.Arm

	// fails on not finding gripper

	r = &inject.Robot{}
	r.ResourceByNameFunc = func(resource.Name) (interface{}, bool) {
		return nil, false
	}
	mgs, err := objectmanipulation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = mgs.DoGrab(context.Background(), "fakeGripper", "fakeArm", "fakeCamera", &r3.Vector{10.0, 10.0, 10.0})
	test.That(t, err, test.ShouldNotBeNil)

	// fails when gripper fails to open
	r = &inject.Robot{}
	_arm = &inject.Arm{}
	_gripper = &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return errors.New("failure to open")
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

	_gripper.OpenFunc = func(ctx context.Context) error {
		return nil
	}

	// can't move gripper with respect to gripper
	_, err = mgs.DoGrab(context.Background(), "fakeGripper", "fakeArm", "fakeGripper", &r3.Vector{0, 0, 200})
	test.That(t, err, test.ShouldBeError, "cannot move gripper with respect to gripper frame, gripper will always be at its own origin")
}

func TestDoGrab(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/moving_arm.json", logger)
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

	cfg, err := config.Read(ctx, "data/fake_tomato.json", logger)
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := objectmanipulation.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	_, err = svc.DoGrab(ctx, "gr", "a", "c", &r3.Vector{-20, -30, -40})
	test.That(t, err, test.ShouldBeNil)

	// remove after this
	theArm, _ := arm.FromRobot(myRobot, "a")
	temp, _ := theArm.GetJointPositions(ctx)
	logger.Debugf("end arm position; %v", temp)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return svc1, true
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	result, err := svc.DoGrab(context.Background(), "", "", "", &r3.Vector{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, success)
	test.That(t, svc1.grabCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return "not object manipulation", true
	}

	svc, err = objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "expected implementation of objectmanipulation.Service")
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, bool) {
		return nil, false
	}

	svc, err = objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "not found")
	test.That(t, svc, test.ShouldBeNil)
}

const success = false

type mock struct {
	objectmanipulation.Service

	grabCount int
}

func (m *mock) DoGrab(ctx context.Context, gripperName, armName, cameraName string, cameraPoint *r3.Vector) (bool, error) {
	m.grabCount++
	return success, nil
}
