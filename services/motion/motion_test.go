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
	"go.viam.com/rdk/registry"
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
		_, err = ms.PlanAndMove(context.Background(), camera.Named("fake"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on disconnected supplemental frames in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
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
		_, err = ms.PlanAndMove(context.Background(), arm.Named("arm1"), poseInFrame, worldState)
		test.That(t, err, test.ShouldBeError, framesystemparts.NewMissingParentError("frame2", "noParent"))
	})
}

func TestMove1(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/moving_arm.json")

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.PlanAndMove(context.Background(), gripper.Named("pieceGripper"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.PlanAndMove(context.Background(), gripper.Named("pieceArm"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.PlanAndMove(context.Background(), gripper.Named("pieceGripper"), grabPose, &commonpb.WorldState{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
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
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{-20, -130, -40}))
		_, err = ms.PlanAndMove(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveWithObstacles(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/moving_arm.json")

	t.Run("check a movement that should not succeed due to obstacles", func(t *testing.T) {
		testPose1 := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 370})
		testPose2 := spatialmath.NewPoseFromPoint(r3.Vector{300, 300, -3500})
		_ = testPose2
		grabPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{-600, -400, 460}))
		obsMsgs := []*commonpb.GeometriesInFrame{
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose2),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 20,
								Y: 40,
								Z: 40,
							}},
						},
					},
				},
			},
			{
				ReferenceFrame: "world",
				Geometries: []*commonpb.Geometry{
					{
						Center: spatialmath.PoseToProtobuf(testPose1),
						GeometryType: &commonpb.Geometry_Box{
							Box: &commonpb.RectangularPrism{DimsMm: &commonpb.Vector3{
								X: 2000,
								Y: 2000,
								Z: 20,
							}},
						},
					},
				},
			},
		}
		_ = obsMsgs
		_, err = ms.PlanAndMove(context.Background(), gripper.Named("pieceArm"), grabPose, &commonpb.WorldState{Obstacles: obsMsgs})
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestMoveSingleComponent(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/moving_arm.json")

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-25, 30, -200}))
		_, err = ms.MoveSingleComponent(context.Background(), arm.Named("pieceArm"), grabPose, &commonpb.WorldState{})
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("fails due to gripper not being an arm", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-20, -30, -40}))
		_, err = ms.MoveSingleComponent(context.Background(), gripper.Named("pieceGripper"), grabPose, &commonpb.WorldState{})
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldNotBeNil)
	})

	ms = setupMotionServiceFromConfig(t, "data/moving_arm.json")

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		testPose := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)

		transformMsgs := []*commonpb.Transform{
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

		poseToGrab := spatialmath.NewPoseFromOrientation(
			r3.Vector{X: -20., Y: 0., Z: -800.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 1., RY: 0., RZ: 0.},
		)

		grabPose := referenceframe.NewPoseInFrame("testFrame2", poseToGrab)
		_, err = ms.MoveSingleComponent(context.Background(), arm.Named("pieceArm"), grabPose, worldState)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms := setupMotionServiceFromConfig(t, "data/fake_tomato.json")
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-0, -30, -50}))
	_, err = ms.PlanAndMove(context.Background(), gripper.Named("gr"), grabPose, &commonpb.WorldState{})
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

	testPose := spatialmath.NewPoseFromOrientation(
		r3.Vector{X: 0., Y: 0., Z: 0.},
		&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
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
	grabCount   int
	name        string
	reconfCount int
}

func (m *mock) PlanAndMove(
	ctx context.Context,
	gripperName resource.Name,
	grabPose *referenceframe.PoseInFrame,
	worldState *commonpb.WorldState,
) (bool, error) {
	m.grabCount++
	return false, nil
}

func (m *mock) MoveSingleComponent(
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

func (m *mock) Close(ctx context.Context) error {
	m.reconfCount++
	return nil
}

func TestFromRobot(t *testing.T) {
	r, svc1 := setupInjectRobot()

	svc, err := motion.FromRobot(r)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, svc, test.ShouldNotBeNil)

	grabPose := referenceframe.NewPoseInFrame("", spatialmath.NewZeroPose())
	result, err := svc.PlanAndMove(context.Background(), gripper.Named("fake"), grabPose, &commonpb.WorldState{})
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

func TestRegisteredReconfigurable(t *testing.T) {
	s := registry.ResourceSubtypeLookup(motion.Subtype)
	test.That(t, s, test.ShouldNotBeNil)
	r := s.Reconfigurable
	test.That(t, r, test.ShouldNotBeNil)
}

func TestWrapWithReconfigurable(t *testing.T) {
	svc := &mock{name: "svc1"}
	reconfSvc1, err := motion.WrapWithReconfigurable(svc)
	test.That(t, err, test.ShouldBeNil)

	_, err = motion.WrapWithReconfigurable(nil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnimplementedInterfaceError("motion.Service", nil))

	reconfSvc2, err := motion.WrapWithReconfigurable(reconfSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldEqual, reconfSvc1)
}

func TestReconfigurable(t *testing.T) {
	actualSvc1 := &mock{name: "svc1"}
	reconfSvc1, err := motion.WrapWithReconfigurable(actualSvc1)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldNotBeNil)

	actualArm2 := &mock{name: "svc2"}
	reconfSvc2, err := motion.WrapWithReconfigurable(actualArm2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc2, test.ShouldNotBeNil)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 0)

	err = reconfSvc1.Reconfigure(context.Background(), reconfSvc2)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, reconfSvc1, test.ShouldResemble, reconfSvc2)
	test.That(t, actualSvc1.reconfCount, test.ShouldEqual, 1)

	err = reconfSvc1.Reconfigure(context.Background(), nil)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err, test.ShouldBeError, rutils.NewUnexpectedTypeError(reconfSvc1, nil))
}
