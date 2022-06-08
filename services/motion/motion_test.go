package motion_test

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/component/arm"
	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/component/gripper"
	// register.
	_ "go.viam.com/rdk/component/register"
	"go.viam.com/rdk/config"
	commonpb "go.viam.com/rdk/proto/api/common/v1"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	framesystemparts "go.viam.com/rdk/robot/framesystem/parts"
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
	t.Run("fail on disconnected supplemental frames in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromAxisAngle(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			r3.Vector{X: 0., Y: 1., Z: 0.},
			math.Pi/2,
		)
		transformMsgs := []*commonpb.Transform{
			{
				ReferenceFrame: "frame2",
				PoseInObserverFrame: &commonpb.PoseInFrame{
					ReferenceFrame: "noParent",
					Pose:           spatialmath.PoseToProtobuf(testPose),
				},
			},
		}
		worldState := &commonpb.WorldState{
			Transforms: transformMsgs,
		}
		poseInFrame := referenceframe.NewPoseInFrame("frame2", spatialmath.NewZeroPose())
		_, err = ms.Move(context.Background(), arm.Named("arm1"), poseInFrame, worldState)
		test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("frame2", "noParent"))
	})
}

func TestMove(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/moving_arm.json")

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromAxisAngle(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			r3.Vector{X: 0., Y: 1., Z: 0.},
			math.Pi/2,
		)

		transformMsgs := []*commonpb.Transform{
			{
				ReferenceFrame: "testFrame",
				PoseInObserverFrame: &commonpb.PoseInFrame{
					ReferenceFrame: "pieceArm",
					Pose:           spatialmath.PoseToProtobuf(testPose),
				},
			},
			{
				ReferenceFrame: "testFrame2",
				PoseInObserverFrame: &commonpb.PoseInFrame{
					ReferenceFrame: "world",
					Pose:           spatialmath.PoseToProtobuf(testPose),
				},
			},
		}
		worldState := &commonpb.WorldState{
			Transforms: transformMsgs,
		}
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState)
	})
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

	pose, err := ms.GetPose(context.Background(), arm.Named("gantry1"), "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "gantry1", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("gantry1"), "gantry1", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "arm1", nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.FrameName(), test.ShouldEqual, "arm1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	testPose := spatialmath.NewPoseFromAxisAngle(
		r3.Vector{X: 0., Y: 0., Z: 0.},
		r3.Vector{X: 0., Y: 1., Z: 0.},
		math.Pi/2,
	)
	transformMsgs := []*commonpb.Transform{
		{
			ReferenceFrame: "testFrame",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "world",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
		{
			ReferenceFrame: "testFrame2",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "testFrame",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}
	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame2", transformMsgs)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, -501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, -300)
	test.That(t, pose.Pose().Orientation().AxisAngles().RX, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().RY, test.ShouldEqual, -1)
	test.That(t, pose.Pose().Orientation().AxisAngles().RZ, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().Theta, test.ShouldAlmostEqual, math.Pi/2)

	transformMsgs = []*commonpb.Transform{
		{
			ReferenceFrame: "testFrame",
			PoseInObserverFrame: &commonpb.PoseInFrame{
				ReferenceFrame: "noParent",
				Pose:           spatialmath.PoseToProtobuf(testPose),
			},
		},
	}
	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame", transformMsgs)
	test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("testFrame", "noParent"))
	test.That(t, pose, test.ShouldBeNil)
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
	supplementalTransforms []*commonpb.Transform,
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
