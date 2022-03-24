package motion_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/gripper"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func TestMoveFailures(t *testing.T) {
	logger := golog.NewTestLogger(t)

	t.Run("fail on not finding gripper", func(t *testing.T) {
		// setup robot and service
		r := &inject.Robot{}
		r.ResourceByNameFunc = func(n resource.Name) (interface{}, error) {
			return nil, rutils.NewResourceNotFoundError(n)
		}
		r.LoggerFunc = func() golog.Logger {
			return logger
		}
		r.ResourceNamesFunc = func() []resource.Name {
			return []resource.Name{}
		}
		ms, err := motion.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		// try move
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
		name, err := resource.NewFromString("fakeCamera")
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.Move(context.Background(), name, grabPose, []*referenceframe.GeometriesInFrame{})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on moving gripper with respect to gripper", func(t *testing.T) {
		// setup robot and service
		_arm := &inject.Arm{}
		_gripper := &inject.Gripper{}
		r := &inject.Robot{}
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
		ms, err := motion.New(context.Background(), r, config.Service{}, logger)
		test.That(t, err, test.ShouldBeNil)

		// try move
		badGrabPose := referenceframe.NewPoseInFrame("fakeGripper", spatialmath.NewZeroPose())
		name, err := resource.NewFromString("fakeGripper")
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.Move(context.Background(), name, badGrabPose, []*referenceframe.GeometriesInFrame{})
		test.That(t, err, test.ShouldBeError, "cannot move component with respect to its own frame, will always be at its own origin")
	})
}

func TestMove(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/moving_arm.json", logger)
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := motion.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	name, err := resource.NewFromString("pieceGripper")
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.Move(ctx, name, grabPose, []*referenceframe.GeometriesInFrame{})
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

	svc, err := motion.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	name, err := resource.NewFromString("gr")
	test.That(t, err, test.ShouldBeNil)
	_, err = svc.Move(ctx, name, grabPose, []*referenceframe.GeometriesInFrame{})
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

	svc, err := motion.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	name, err := resource.NewFromString("")
	test.That(t, err, test.ShouldBeNil)
	result, err := svc.Move(context.Background(), name, grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, success)
	test.That(t, svc1.grabCount, test.ShouldEqual, 1)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return "not motion", nil
	}

	svc, err = motion.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", "string"))
	test.That(t, svc, test.ShouldBeNil)

	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return nil, rutils.NewResourceNotFoundError(name)
	}

	svc, err = motion.FromRobot(r)
	test.That(t, err, test.ShouldBeError, rutils.NewResourceNotFoundError(motion.Name))
	test.That(t, svc, test.ShouldBeNil)
}

const success = false

type mock struct {
	motion.Service
	grabCount int
}

func (m *mock) Move(
	ctx context.Context,
	gripperName string,
	grabPose *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	m.grabCount++
	return success, nil
}
