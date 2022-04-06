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
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rutils "go.viam.com/rdk/utils"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) motion.Service {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer myRobot.Close(context.Background())
	svc, err := motion.New(ctx, myRobot, config.Service{}, logger)
	test.That(t, err, test.ShouldBeNil)
	return svc
}

func TestMoveFailures(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/arm_gantry.json")
	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
		_, err = ms.Move(context.Background(), camera.Named("fake"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldNotBeNil)
	})
	t.Run("fail on moving gripper with respect to gripper", func(t *testing.T) {
		badGrabPose := referenceframe.NewPoseInFrame("arm1", spatialmath.NewZeroPose())
		_, err = ms.Move(context.Background(), gripper.Named("arm1"), badGrabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeError, "cannot move component with respect to its own frame, will always be at its own origin")
	})
}

func TestMove(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/moving_arm.json")
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, &commonpb.WorldState{})
	test.That(t, err, test.ShouldBeNil)
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/fake_tomato.json")
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
	_, err = ms.Move(context.Background(), gripper.Named("gr"), grabPose, &commonpb.WorldState{})
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/arm_gantry.json")

	pose, err := ms.GetPose(context.Background(), arm.Named("gantry1"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "gantry1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("gantry1"), "gantry1")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "arm1")
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
	worldState *commonpb.WorldState,
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

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := motion.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	result, err := svc.Move(context.Background(), gripper.Named("fake"), grabPose, &commonpb.WorldState{})
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
