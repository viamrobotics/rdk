package builtin

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/arm"
	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"

	// register.
	commonpb "go.viam.com/api/common/v1"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/referenceframe"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"

	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) (motion.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := motion.FromRobot(myRobot, "builtin")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func TestMoveFailures(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()
	ctx := context.Background()
	t.Run("fail on not finding gripper", func(t *testing.T) {
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{10.0, 10.0, 10.0}))
		_, err = ms.Move(ctx, camera.Named("fake"), grabPose, nil, nil, nil)
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("fail on disconnected supplemental frames in world state", func(t *testing.T) {
		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)
		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame("noParent", testPose, "frame2", nil),
		}
		worldState, err := referenceframe.NewWorldState(nil, transforms)
		test.That(t, err, test.ShouldBeNil)
		poseInFrame := referenceframe.NewPoseInFrame("frame2", spatialmath.NewZeroPose())
		_, err = ms.Move(ctx, arm.Named("arm1"), poseInFrame, worldState, nil, nil)
		test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("frame2", "noParent"))
	})
}

func TestMove(t *testing.T) {
	var err error
	ctx := context.Background()

	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceGripper"), grabPose, nil, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceArm"), grabPose, nil, nil, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceGripper"), grabPose, nil, nil, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		testPose := spatialmath.NewPose(
			r3.Vector{X: 1., Y: 2., Z: 3.},
			&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.},
		)

		transforms := []*referenceframe.LinkInFrame{
			referenceframe.NewLinkInFrame(referenceframe.World, testPose, "testFrame2", nil),
			referenceframe.NewLinkInFrame("pieceArm", testPose, "testFrame", nil),
		}

		worldState, err := referenceframe.NewWorldState(nil, transforms)
		test.That(t, err, test.ShouldBeNil)
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{-20, -130, -40}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveWithObstacles(t *testing.T) {
	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()

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
		worldState, err := referenceframe.WorldStateFromProtobuf(&commonpb.WorldState{Obstacles: obsMsgs})
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.Move(context.Background(), gripper.Named("pieceArm"), grabPose, worldState, nil, nil)
		// This fails due to a large obstacle being in the way
		test.That(t, err, test.ShouldNotBeNil)
	})
}

func TestMoveSingleComponent(t *testing.T) {
	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()

	grabPose := spatialmath.NewPoseFromPoint(r3.Vector{-25, 30, 0})
	t.Run("succeeds when all frame info in config", func(t *testing.T) {
		_, err := ms.MoveSingleComponent(
			context.Background(),
			arm.Named("pieceArm"),
			referenceframe.NewPoseInFrame("c", grabPose),
			nil,
			map[string]interface{}{},
		)
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("fails due to gripper not being an arm", func(t *testing.T) {
		_, err := ms.MoveSingleComponent(
			context.Background(),
			gripper.Named("pieceGripper"),
			referenceframe.NewPoseInFrame("c", grabPose),
			nil,
			map[string]interface{}{},
		)
		// Gripper is not an arm and cannot move
		test.That(t, err, test.ShouldNotBeNil)
	})

	t.Run("succeeds with supplemental info in world state", func(t *testing.T) {
		worldState, err := referenceframe.NewWorldState(
			nil,
			[]*referenceframe.LinkInFrame{referenceframe.NewLinkInFrame("c", spatialmath.NewZeroPose(), "testFrame2", nil)},
		)
		test.That(t, err, test.ShouldBeNil)
		_, err = ms.MoveSingleComponent(
			context.Background(),
			arm.Named("pieceArm"),
			referenceframe.NewPoseInFrame("testFrame2", grabPose),
			worldState,
			map[string]interface{}{},
		)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveOnMap(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	injectSlam := inject.NewSLAMService("test_slam")

	const chunkSizeBytes = 1 * 1024 * 1024

	injectSlam.GetPointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		path := filepath.Clean(artifact.MustPath("pointcloud/octagonspace.pcd"))
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		chunk := make([]byte, chunkSizeBytes)
		f := func() ([]byte, error) {
			bytesRead, err := file.Read(chunk)
			if err != nil {
				defer utils.UncheckedErrorFunc(file.Close)
				return nil, err
			}
			return chunk[:bytesRead], err
		}
		return f, nil
	}
	injectSlam.GetPositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}

	cfg := resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 100}},
	}

	fakeBase, err := fake.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	ms, err := NewBuiltIn(
		ctx,
		resource.Dependencies{injectSlam.Name(): injectSlam, fakeBase.Name(): fakeBase},
		resource.Config{
			ConvertedAttributes: &Config{},
		},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	// goal x-position of 1.32m is scaled to be in mm
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1.32 * 1000, Y: 0})

	t.Run("check that path is planned around obstacle", func(t *testing.T) {
		t.Parallel()
		path, _, err := ms.(*builtIn).planMoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			nil,
		)
		test.That(t, err, test.ShouldBeNil)
		// path of length 2 indicates a path that goes straight through central obstacle
		test.That(t, len(path), test.ShouldBeGreaterThan, 2)
	})

	t.Run("ensure success of movement around obstacle", func(t *testing.T) {
		t.Parallel()
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			nil,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("check that straight line path executes", func(t *testing.T) {
		t.Parallel()
		easyGoal := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test_base"),
			easyGoal,
			slam.Named("test_slam"),
			nil,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})
}

func TestMoveOnGlobe(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	gpsPoint := geo.NewPoint(10, 10)

	// create motion config
	motionCfg := make(map[string]interface{})
	motionCfg["motion_profile"] = "position_only"
	motionCfg["timeout"] = 5.

	// create fake base
	baseCfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	fakeBase, err := fake.NewBase(ctx, nil, baseCfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// create base frame
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"test-base",
		basePose,
		baseSphere,
	)
	test.That(t, err, test.ShouldBeNil)

	// create injected MovementSensor
	injectedMovementSensor := inject.NewMovementSensor("test-gps")
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return gpsPoint, 0, nil
	}

	// create MovementSensor frame
	movementSensorFrame, err := referenceframe.NewStaticFrame(
		"test-gps",
		spatialmath.NewPoseFromPoint(r3.Vector{-10, 0, 0}),
	)
	test.That(t, err, test.ShouldBeNil)

	// create a framesystem
	newFS := referenceframe.NewEmptyFrameSystem("test-FS")
	worldFrame, err := referenceframe.NewStaticFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}))
	test.That(t, err, test.ShouldBeNil)
	newFS.AddFrame(baseFrame, worldFrame)
	newFS.AddFrame(movementSensorFrame, baseFrame)

	// need to create an injected framesystem service
	injectedFS := inject.NewFrameSystemService("fake-FS")
	injectedFS.FrameSystemFunc = func(ctx context.Context, additionalTransforms []*referenceframe.LinkInFrame) (referenceframe.FrameSystem, error) {
		return newFS, nil
	}
	injectedFS.CurrentInputsFunc = func(ctx context.Context) (map[string][]referenceframe.Input, map[string]referenceframe.InputEnabled, error) {
		return referenceframe.StartPositions(newFS), nil, nil
	}

	// create the motion service
	ms, err := NewBuiltIn(
		ctx,
		resource.Dependencies{
			fakeBase.Name():               fakeBase,
			injectedMovementSensor.Name(): injectedMovementSensor,
			injectedFS.Name():             injectedFS,
		},
		resource.Config{
			ConvertedAttributes: &Config{},
		},
		logger,
	)
	test.That(t, err, test.ShouldBeNil)

	gp, _, err := injectedMovementSensor.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	destGP := geo.NewPoint(gp.Lat(), gp.Lng()+0.0000009)

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		plan, _, err := ms.(*builtIn).planMoveOnGlobeNick(
			context.Background(),
			fakeBase.Name(),
			destGP,
			injectedMovementSensor.Name(),
			nil,
			math.NaN(),
			math.NaN(),
			motionCfg,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(plan), test.ShouldEqual, 2)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{5, 50, 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})

		plan, _, err := ms.(*builtIn).planMoveOnGlobeNick(
			context.Background(),
			fakeBase.Name(),
			destGP,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			math.NaN(),
			math.NaN(),
			motionCfg,
		)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("fail because of long wall", func(t *testing.T) {
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{2, 666, 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})

		plan, _, err := ms.(*builtIn).planMoveOnGlobeNick(
			context.Background(),
			fakeBase.Name(),
			destGP,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			math.NaN(),
			math.NaN(),
			motionCfg,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, len(plan), test.ShouldEqual, 0)
	})

	t.Run("relative position and distance are properly calculated", func(t *testing.T) {
		localizer, err := motion.NewLocalizer(ctx, injectedMovementSensor)
		test.That(t, err, test.ShouldBeNil)
		currentPosition, dstPIF, err := ms.(*builtIn).getRelativePositionAndDestination(ctx, localizer, fakeBase.Name(), injectedMovementSensor.Name(), *destGP)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, currentPosition, test.ShouldResemble, r3.Vector{-10, 0, 0})
		test.That(t, spatialmath.R3VectorAlmostEqual(dstPIF.Pose().Point(), r3.Vector{110, 0, 0}, 0.1), test.ShouldBeTrue)
		test.That(t, dstPIF.Parent(), test.ShouldEqual, referenceframe.World)
	})
}

func TestMultiplePieces(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/fake_tomato.json")
	defer teardown()
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{-0, -30, -50}))
	_, err = ms.Move(context.Background(), gripper.Named("gr"), grabPose, nil, nil, nil)
	test.That(t, err, test.ShouldBeNil)
}

func TestGetPose(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/arm_gantry.json")
	defer teardown()

	pose, err := ms.GetPose(context.Background(), arm.Named("gantry1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 1.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, referenceframe.World)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 500)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 300)

	pose, err = ms.GetPose(context.Background(), arm.Named("gantry1"), "gantry1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "gantry1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "arm1", nil, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Parent(), test.ShouldEqual, "arm1")
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, 0)

	testPose := spatialmath.NewPoseFromOrientation(&spatialmath.R4AA{Theta: math.Pi / 2, RX: 0., RY: 1., RZ: 0.})
	transforms := []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame(referenceframe.World, testPose, "testFrame", nil),
		referenceframe.NewLinkInFrame("testFrame", testPose, "testFrame2", nil),
	}

	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame2", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, pose.Pose().Point().X, test.ShouldAlmostEqual, -501.2)
	test.That(t, pose.Pose().Point().Y, test.ShouldAlmostEqual, 0)
	test.That(t, pose.Pose().Point().Z, test.ShouldAlmostEqual, -300)
	test.That(t, pose.Pose().Orientation().AxisAngles().RX, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().RY, test.ShouldEqual, -1)
	test.That(t, pose.Pose().Orientation().AxisAngles().RZ, test.ShouldEqual, 0)
	test.That(t, pose.Pose().Orientation().AxisAngles().Theta, test.ShouldAlmostEqual, math.Pi)

	transforms = []*referenceframe.LinkInFrame{
		referenceframe.NewLinkInFrame("noParent", testPose, "testFrame", nil),
	}
	pose, err = ms.GetPose(context.Background(), arm.Named("arm1"), "testFrame", transforms, map[string]interface{}{})
	test.That(t, err, test.ShouldBeError, referenceframe.NewParentFrameMissingError("testFrame", "noParent"))
	test.That(t, pose, test.ShouldBeNil)
}
