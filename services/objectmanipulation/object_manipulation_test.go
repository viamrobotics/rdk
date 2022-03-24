package objectmanipulation_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/objectmanipulation"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestDoGrabFailures(t *testing.T) {
	cfgService := config.Service{}
	logger := golog.NewTestLogger(t)

	// fails on not finding gripper

	r := &inject.Robot{}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(n)
	}
	mgs, err := objectmanipulation.New(context.Background(), r, cfgService, logger)
	test.That(t, err, test.ShouldBeNil)

	grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
	_, err = mgs.DoGrab(context.Background(), "fakeGripper", grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldNotBeNil)

	// fails when gripper fails to open
	_arm := &inject.Arm{}
	_gripper := &inject.Gripper{}
	_gripper.OpenFunc = func(ctx context.Context) error {
		return errors.New("failure to open")
	}
	r.LoggerFunc = func() golog.Logger {
		return logger
	}
	r.ResourceNamesFunc = func() []resource.Name {
		return []resource.Name{arm.Named("fakeArm"), gripper.Named("fakeGripper")}
	}
	r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
		switch n.Name {
		case "fakeArm":
			return _arm, nil
		case "fakeGripper":
			return _gripper, nil
		default:
			return nil, rutils.NewResourceNotFoundError(n)
		}
	}

	mgs, _ = objectmanipulation.New(context.Background(), r, cfgService, logger)

	_, err = mgs.DoGrab(context.Background(), "fakeGripper", grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldNotBeNil)

	_gripper.OpenFunc = func(ctx context.Context) error {
		return nil
	}

	// can't move gripper with respect to gripper
	badGrabPose := referenceframe.NewPoseInFrame("fakeGripper", spatialmath.NewZeroPose())
	_, err = mgs.DoGrab(context.Background(), "fakeGripper", badGrabPose, []*referenceframe.GeometriesInFrame{})
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

	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	_, err = svc.DoGrab(ctx, "pieceGripper", grabPose, []*referenceframe.GeometriesInFrame{})
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

	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	_, err = svc.DoGrab(ctx, "gr", grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldBeNil)

	// remove after this
	theArm, _ := arm.FromRobot(myRobot, "a")
	temp, _ := theArm.GetJointPositions(ctx)
	logger.Debugf("end arm position; %v", temp)
}

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	result, err := svc.DoGrab(context.Background(), "", grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, success)
	test.That(t, svc1.grabCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not object manipulation", nil
	}

	svc, err = objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("objectmanipulation.Service", "string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(name)
	}

	svc, err = objectmanipulation.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(objectmanipulation.Name))
	test.That(t, svc, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/arm_gantry.json", logger)
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := objectmanipulation.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	pose, err := svc.GetPose(ctx, arm.Named("gantry1"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = svc.GetPose(ctx, arm.Named("arm1"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = svc.GetPose(ctx, arm.Named("arm1"), "gantry1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = svc.GetPose(ctx, arm.Named("gantry1"), "gantry1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = svc.GetPose(ctx, arm.Named("arm1"), "arm1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "arm1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)
}

const success = false

type mock struct {
	objectmanipulation.Service
	grabCount int
}

func (m *mock) DoGrab(
	ctx context.Context,
	gripperName string,
	grabPose *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	m.grabCount++
	return success, nil
}
