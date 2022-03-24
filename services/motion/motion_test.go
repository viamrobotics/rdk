package motion_test

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
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
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/arm_gantry.json", logger)
	test.That(t, err, test.ShouldBeNil)

	r, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer r.Close(context.Background())

	ms, err := motion.New(context.Background(), r, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)

	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
		_, err = ms.Move(context.Background(), camera.Named("fake"), grabPose, []*referenceframe.GeometriesInFrame{})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on moving gripper with respect to gripper", func(t *testing.T) {
		badGrabPose := referenceframe.NewPoseInFrame("arm1", spatialmath.NewZeroPose())
		_, err = ms.Move(context.Background(), gripper.Named("arm1"), badGrabPose, []*referenceframe.GeometriesInFrame{})
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
	_, err = svc.Move(ctx, gripper.Named("pieceGripper"), grabPose, []*referenceframe.GeometriesInFrame{})
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
	_, err = svc.Move(ctx, gripper.Named("gr"), grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldBeNil)

	// remove after this
	theArm, _ := arm.FromRobot(myRobot, "a")
	temp, _ := theArm.GetJointPositions(ctx)
	logger.Debugf("end arm position; %v", temp)
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := motion.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	result, err := svc.Move(context.Background(), gripper.Named("fake"), grabPose, []*referenceframe.GeometriesInFrame{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, result, test.ShouldEqual, false)
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

func TestGetPose(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	cfg, err := config.Read(ctx, "data/arm_gantry.json", logger)
	test.That(t, err, test.ShouldBeNil)

	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())

	svc, err := motion.New(ctx, myRobot, config.Service{}, logger)
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

func setupInjectRobot() (*inject.Robot, *mock) {
	svc1 := &mock{}
	r := &inject.Robot{}
	r.ResourceByNameFunc = func(name resource.Name) (interface{}, error) {
		return svc1, nil
	}
	return r, svc1
}

type mock struct {
	motion.Service
	grabCount int
}

func (m *mock) Move(
	ctx context.Context,
	gripperName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	obstacles []*referenceframe.GeometriesInFrame,
) (bool, error) {
	m.grabCount++
	return false, nil
}

func (m *mock) GetPose(
	ctx context.Context,
	componentName resource.Name,
	destinationFrame string,
) (*referenceframe.PoseInFrame, error) {
	return &referenceframe.PoseInFrame{}, nil
}
