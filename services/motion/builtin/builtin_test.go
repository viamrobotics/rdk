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
	"github.com/pkg/errors"

	// registers all components.
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/test"
	"go.viam.com/utils"
	"go.viam.com/utils/artifact"

	"go.viam.com/rdk/components/arm"
	armFake "go.viam.com/rdk/components/arm/fake"
	ur "go.viam.com/rdk/components/arm/universalrobots"
	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/register"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	rdkutils "go.viam.com/rdk/utils"
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

func getPointCloudMap(path string) (func() ([]byte, error), error) {
	const chunkSizeBytes = 1 * 1024 * 1024
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

func createInjectedMovementSensor(name string, gpsPoint *geo.Point) *inject.MovementSensor {
	injectedMovementSensor := inject.NewMovementSensor(name)
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return gpsPoint, 0, nil
	}
	injectedMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	injectedMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	return injectedMovementSensor
}

func createInjectedSlam(name, pcdPath string) *inject.SLAMService {
	injectSlam := inject.NewSLAMService(name)
	injectSlam.GetPointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return getPointCloudMap(filepath.Clean(artifact.MustPath(pcdPath)))
	}
	injectSlam.GetPositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}
	return injectSlam
}

func createBaseLink(t *testing.T, baseName string) *referenceframe.LinkInFrame {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		baseName,
		baseSphere,
	)
	return baseLink
}

func createFrameSystemService(
	ctx context.Context,
	deps resource.Dependencies,
	fsParts []*referenceframe.FrameSystemPart,
	logger golog.Logger,
) (framesystem.Service, error) {
	fsSvc, err := framesystem.New(ctx, deps, logger)
	if err != nil {
		return nil, err
	}
	conf := resource.Config{
		ConvertedAttributes: &framesystem.Config{Parts: fsParts},
	}
	if err := fsSvc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	deps[fsSvc.Name()] = fsSvc

	return fsSvc, nil
}

func createMoveOnGlobeEnvironment(ctx context.Context, t *testing.T, gpsPoint *geo.Point) (
	*inject.MovementSensor, framesystem.Service, base.Base, motion.Service,
) {
	logger := golog.NewTestLogger(t)

	// create fake base
	baseCfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	fakeBase, err := baseFake.NewBase(ctx, nil, baseCfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// create base link
	baseLink := createBaseLink(t, "test-base")

	// create injected MovementSensor
	injectedMovementSensor := createInjectedMovementSensor("test-gps", gpsPoint)

	// create MovementSensor link
	movementSensorLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{-10, 0, 0}),
		"test-gps",
		nil,
	)

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: movementSensorLink},
		{FrameConfig: baseLink},
	}
	deps := resource.Dependencies{
		fakeBase.Name():               fakeBase,
		injectedMovementSensor.Name(): injectedMovementSensor,
	}

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	return injectedMovementSensor, fsSvc, fakeBase, ms
}

func createMoveOnMapEnvironment(ctx context.Context, t *testing.T, pcdPath string) motion.Service {
	injectSlam := createInjectedSlam("test_slam", pcdPath)

	cfg := resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 100}},
	}
	logger := golog.NewTestLogger(t)
	fakeBase, err := baseFake.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	deps := resource.Dependencies{injectSlam.Name(): injectSlam, fakeBase.Name(): fakeBase}
	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	return ms
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
		_, err = ms.Move(ctx, arm.Named("pieceArm"), grabPose, nil, nil, map[string]interface{}{})
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

func TestMoveOnMapLongDistance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// goal position is scaled to be in mm
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: -32.508 * 1000, Y: -2.092 * 1000})

	t.Run("test cbirrt planning on office map", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd")
		extra := make(map[string]interface{})
		extra["planning_alg"] = "cbirrt"

		path, _, err := ms.(*builtIn).planMoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			kinematicbase.NewKinematicBaseOptions(),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(path), test.ShouldBeGreaterThan, 2)
	})

	t.Run("test rrtstar planning on office map", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd")
		extra := make(map[string]interface{})
		extra["planning_alg"] = "rrtstar"

		path, _, err := ms.(*builtIn).planMoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			kinematicbase.NewKinematicBaseOptions(),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(path), test.ShouldBeGreaterThan, 2)
	})
}

func TestMoveOnMap(t *testing.T) {
	t.Skip() // RSDK-4279
	t.Parallel()
	ctx := context.Background()
	// goal x-position of 1.32m is scaled to be in mm
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1.32 * 1000, Y: 0})

	t.Run("check that path is planned around obstacle", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		extra := make(map[string]interface{})
		extra["motion_profile"] = "orientation"
		path, _, err := ms.(*builtIn).planMoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			kinematicbase.NewKinematicBaseOptions(),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		// path of length 2 indicates a path that goes straight through central obstacle
		test.That(t, len(path), test.ShouldBeGreaterThan, 2)
		// every waypoint should have the form [x,y,theta]
		test.That(t, len(path[0]), test.ShouldEqual, 3)
	})

	t.Run("ensure success of movement around obstacle", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
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
		ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
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

	t.Run("check that position-only mode returns 2D plan", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		extra := make(map[string]interface{})
		extra["motion_profile"] = "position_only"
		path, _, err := ms.(*builtIn).planMoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			kinematicbase.NewKinematicBaseOptions(),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		// every waypoint should have the form [x,y]
		test.That(t, len(path[0]), test.ShouldEqual, 2)
	})

	t.Run("check that position-only mode executes", func(t *testing.T) {
		t.Parallel()
		ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		extra := make(map[string]interface{})
		extra["motion_profile"] = "position_only"
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test_base"),
			goal,
			slam.Named("test_slam"),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})
}

func TestMoveOnMapTimeout(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	cfg, err := config.Read(ctx, "../data/real_wheeled_base.json", logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, myRobot.Close(context.Background()), test.ShouldBeNil)
	}()

	injectSlam := createInjectedSlam("test_slam", "pointcloud/octagonspace.pcd")

	realBase, err := base.FromRobot(myRobot, "test_base")
	test.That(t, err, test.ShouldBeNil)

	deps := resource.Dependencies{
		injectSlam.Name(): injectSlam,
		realBase.Name():   realBase,
	}
	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	easyGoal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1001, Y: 1001})
	success, err := ms.MoveOnMap(
		context.Background(),
		base.Named("test_base"),
		easyGoal,
		slam.Named("test_slam"),
		nil,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, success, test.ShouldBeFalse)
}

func TestPlanMoveOnGlobe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gpsPoint := geo.NewPoint(-70, 40)

	motionCfg := make(map[string]interface{})
	// only generate plan for x and y, exclude theta
	motionCfg["motion_profile"] = "position_only"
	// fail if we don't find a plan in 5 seconds
	motionCfg["timeout"] = 5.

	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	expectedDst := r3.Vector{380, 0, 0}

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint)
		solutionMap, kb, err := ms.(*builtIn).planMoveOnGlobe(
			context.Background(),
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			nil,
			kinematicbase.NewKinematicBaseOptions(),
			motionCfg,
		)
		test.That(t, err, test.ShouldBeNil)
		plan, err := motionplan.FrameStepsFromRobotPath(kb.Kinematics().Name(), solutionMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(plan), test.ShouldEqual, 2)
		test.That(t, plan[1][0].Value, test.ShouldAlmostEqual, expectedDst.X, 10)
		test.That(t, plan[1][1].Value, test.ShouldAlmostEqual, expectedDst.Y, 10)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint)
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{5, 50, 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})

		solutionMap, kb, err := ms.(*builtIn).planMoveOnGlobe(
			context.Background(),
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			kinematicbase.NewKinematicBaseOptions(),
			motionCfg,
		)
		test.That(t, err, test.ShouldBeNil)
		plan, err := motionplan.FrameStepsFromRobotPath(kb.Kinematics().Name(), solutionMap)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(plan), test.ShouldBeGreaterThan, 2)
		test.That(t, plan[len(plan)-1][0].Value, test.ShouldAlmostEqual, expectedDst.X, 10)
		test.That(t, plan[len(plan)-1][1].Value, test.ShouldAlmostEqual, expectedDst.Y, 10)
	})

	t.Run("fail because of obstacle", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint)

		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{2, 6660, 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})

		plan, _, err := ms.(*builtIn).planMoveOnGlobe(
			context.Background(),
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			kinematicbase.NewKinematicBaseOptions(),
			motionCfg,
		)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, len(plan), test.ShouldEqual, 0)
	})

	t.Run("check offset constructed correctly", func(t *testing.T) {
		t.Parallel()
		_, fsSvc, _, _ := createMoveOnGlobeEnvironment(ctx, t, gpsPoint)
		baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
		movementSensorToBase, err := fsSvc.TransformPose(ctx, baseOrigin, "test-gps", nil)
		if err != nil {
			movementSensorToBase = baseOrigin
		}
		test.That(t, movementSensorToBase.Pose().Point(), test.ShouldResemble, r3.Vector{10, 0, 0})
	})
}

func TestCheckPlan(t *testing.T) {
	ctx := context.Background()

	// orign as gps point
	gpsPoint := geo.NewPoint(-70, 40)

	// create env
	injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint)

	motionCfg := make(map[string]interface{})
	// fail if we don't find a plan in 5 seconds
	motionCfg["timeout"] = 5.

	// get plan and kinematic base
	plan, kinBase, err := ms.(*builtIn).planMoveOnGlobe(
		context.Background(),
		fakeBase.Name(),
		geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5),
		injectedMovementSensor.Name(),
		nil,
		kinematicbase.NewKinematicBaseOptions(),
		motionCfg,
	)
	test.That(t, err, test.ShouldBeNil)

	newFS := referenceframe.NewEmptyFrameSystem("test-fs")
	newFS.AddFrame(kinBase.Kinematics(), newFS.World())

	errorState := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})

	t.Run("check plan without obstacles - ensure success", func(t *testing.T) {
		b, err := motionplan.CheckPlan(kinBase.Kinematics(), plan, nil, newFS, errorState)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b, test.ShouldBeTrue)
	})
	t.Run("check plan with a blocking obstacle - ensure failure", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{150, 0, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		b, err := motionplan.CheckPlan(kinBase.Kinematics(), plan, worldState, newFS, errorState)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, b, test.ShouldBeFalse)
	})

	t.Run("non nil error state - ensure success", func(t *testing.T) {
		errorState := spatialmath.NewPoseFromPoint(r3.Vector{0, 26, 0})

		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{150, 0, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		b, err := motionplan.CheckPlan(kinBase.Kinematics(), plan, worldState, newFS, errorState)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b, test.ShouldBeTrue)
	})

	// create camera_origin frame
	cameraOriginFrame, err := referenceframe.NewStaticFrame("camera-origin", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = newFS.AddFrame(cameraOriginFrame, kinBase.Kinematics())
	test.That(t, err, test.ShouldBeNil)

	// create camera geometry
	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{1, 1, 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	// create cameraFrame and add to framesystem
	cameraFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"camera-frame", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}), cameraGeom,
	)
	test.That(t, err, test.ShouldBeNil)
	err = newFS.AddFrame(cameraFrame, cameraOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	t.Run("ensure transforms of obstacles works - no collision", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{150, -6, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		b, err := motionplan.CheckPlan(kinBase.Kinematics(), plan, worldState, newFS, errorState)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, b, test.ShouldBeTrue)
	})
	t.Run("ensure transforms of obstacles works - collision", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{150, 30, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		b, err := motionplan.CheckPlan(kinBase.Kinematics(), plan, worldState, newFS, errorState)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, b, test.ShouldBeFalse)
	})
}

func TestArmGantryPlanCheck(t *testing.T) {
	logger := golog.NewTestLogger(t)
	fs := referenceframe.NewEmptyFrameSystem("test")

	gantryOffset, err := referenceframe.NewStaticFrame("gantryOffset", spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}))
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryOffset, fs.World())

	gantryX, err := referenceframe.NewTranslationalFrame("gantryX", r3.Vector{1, 0, 0}, referenceframe.Limit{math.Inf(-1), math.Inf(1)})
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(gantryX, gantryOffset)

	modelXarm, err := referenceframe.ParseModelJSONFile(rdkutils.ResolveFile("components/arm/xarm/xarm6_kinematics.json"), "")
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(modelXarm, gantryX)

	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 407, Y: 0, Z: 112})

	plan, err := motionplan.PlanMotion(
		context.Background(),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, goal),
		fs.Frame("xArm6"),
		referenceframe.StartPositions(fs),
		fs,
		nil,
		nil,
		nil,
	)
	test.That(t, err, test.ShouldBeNil)

	errorState := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})

	t.Run("check plan with no obstacles", func(t *testing.T) {
		b, err := motionplan.CheckPlan(fs.Frame("xArm6"), plan, nil, fs, errorState)
		test.That(t, b, test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("check plan with obstacle", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{400, 0, 112}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)

		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		b, err := motionplan.CheckPlan(fs.Frame("xArm6"), plan, worldState, fs, errorState)
		test.That(t, b, test.ShouldBeFalse)
		test.That(t, err, test.ShouldNotBeNil)
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

func TestStoppableMoveFunctions(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	failToReachGoalError := errors.New("failed to reach goal")
	calledStopFunc := false
	testIfStoppable := func(t *testing.T, success bool, err error) {
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldEqual, failToReachGoalError)
		test.That(t, success, test.ShouldBeFalse)
		test.That(t, calledStopFunc, test.ShouldBeTrue)
	}

	t.Run("successfully stop arms", func(t *testing.T) {
		armName := "test-arm"
		injectArmName := arm.Named(armName)
		goal := referenceframe.NewPoseInFrame(
			armName,
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -10, Z: -10}),
		)

		// Create an injected Arm
		armCfg := resource.Config{
			Name:  armName,
			API:   arm.API,
			Model: resource.DefaultModelFamily.WithModel("ur5e"),
			ConvertedAttributes: &armFake.Config{
				ArmModel: "ur5e",
			},
			Frame: &referenceframe.LinkConfig{
				Parent: "world",
			},
		}

		fakeArm, err := armFake.NewArm(ctx, nil, armCfg, logger)
		test.That(t, err, test.ShouldBeNil)

		injectArm := &inject.Arm{
			Arm: fakeArm,
		}
		injectArm.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			calledStopFunc = true
			return nil
		}
		injectArm.GoToInputsFunc = func(ctx context.Context, goal []referenceframe.Input) error {
			return failToReachGoalError
		}
		injectArm.ModelFrameFunc = func() referenceframe.Model {
			model, _ := ur.MakeModelFrame("ur5e")
			return model
		}
		injectArm.MoveToPositionFunc = func(ctx context.Context, to spatialmath.Pose, extra map[string]interface{}) error {
			return failToReachGoalError
		}

		// create arm link
		armLink := referenceframe.NewLinkInFrame(
			referenceframe.World,
			spatialmath.NewZeroPose(),
			armName,
			nil,
		)

		// Create a motion service
		fsParts := []*referenceframe.FrameSystemPart{
			{
				FrameConfig: armLink,
				ModelFrame:  injectArm.ModelFrameFunc(),
			},
		}
		deps := resource.Dependencies{
			injectArmName: injectArm,
		}

		_, err = createFrameSystemService(ctx, deps, fsParts, logger)
		test.That(t, err, test.ShouldBeNil)

		conf := resource.Config{ConvertedAttributes: &Config{}}
		ms, err := NewBuiltIn(ctx, deps, conf, logger)
		test.That(t, err, test.ShouldBeNil)

		t.Run("stop during Move(...) call", func(t *testing.T) {
			calledStopFunc = false
			success, err := ms.Move(ctx, injectArmName, goal, nil, nil, nil)
			testIfStoppable(t, success, err)
		})
	})

	t.Run("successfully stop kinematic bases", func(t *testing.T) {
		// Create an injected Base
		baseName := "test-base"

		geometry, err := (&spatialmath.GeometryConfig{R: 20}).ParseConfig()
		test.That(t, err, test.ShouldBeNil)

		injectBase := inject.NewBase(baseName)
		injectBase.GeometriesFunc = func(ctx context.Context) ([]spatialmath.Geometry, error) {
			return []spatialmath.Geometry{geometry}, nil
		}
		injectBase.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (base.Properties, error) {
			return base.Properties{
				TurningRadiusMeters: 0,
				WidthMeters:         600 * 0.001,
			}, nil
		}
		injectBase.StopFunc = func(ctx context.Context, extra map[string]interface{}) error {
			calledStopFunc = true
			return nil
		}
		injectBase.SpinFunc = func(ctx context.Context, angleDeg, degsPerSec float64, extra map[string]interface{}) error {
			return failToReachGoalError
		}
		injectBase.MoveStraightFunc = func(ctx context.Context, distanceMm int, mmPerSec float64, extra map[string]interface{}) error {
			return failToReachGoalError
		}
		injectBase.SetVelocityFunc = func(ctx context.Context, linear, angular r3.Vector, extra map[string]interface{}) error {
			return failToReachGoalError
		}

		// Create a base link
		baseLink := createBaseLink(t, baseName)

		t.Run("stop during MoveOnGlobe(...) call", func(t *testing.T) {
			calledStopFunc = false
			gpsPoint := geo.NewPoint(-70, 40)

			// Create an injected MovementSensor
			movementSensorName := "test-gps"
			injectMovementSensor := createInjectedMovementSensor(movementSensorName, gpsPoint)

			// Create a MovementSensor link
			movementSensorLink := referenceframe.NewLinkInFrame(
				baseLink.Name(),
				spatialmath.NewPoseFromPoint(r3.Vector{-10, 0, 0}),
				movementSensorName,
				nil,
			)

			// Create a motion service
			fsParts := []*referenceframe.FrameSystemPart{
				{FrameConfig: movementSensorLink},
				{FrameConfig: baseLink},
			}
			deps := resource.Dependencies{
				injectBase.Name():           injectBase,
				injectMovementSensor.Name(): injectMovementSensor,
			}

			_, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)

			conf := resource.Config{ConvertedAttributes: &Config{}}
			ms, err := NewBuiltIn(ctx, deps, conf, logger)
			test.That(t, err, test.ShouldBeNil)

			goal := geo.NewPoint(gpsPoint.Lat()+1e-4, gpsPoint.Lng()+1e-4)
			motionCfg := motion.MotionConfiguration{
				PlanDeviationM: 10,
				LinearMPerSec:  10,
			}
			success, err := ms.MoveOnGlobe(
				ctx, injectBase.Name(), goal, 0, injectMovementSensor.Name(),
				nil, &motionCfg, nil,
			)
			testIfStoppable(t, success, err)
		})

		t.Run("stop during MoveOnMap(...) call", func(t *testing.T) {
			calledStopFunc = false
			slamName := "test-slam"

			// Create an injected SLAM
			injectSlam := createInjectedSlam(slamName, "pointcloud/octagonspace.pcd")

			// Create a motion service
			deps := resource.Dependencies{
				injectBase.Name(): injectBase,
				injectSlam.Name(): injectSlam,
			}

			ms, err := NewBuiltIn(
				ctx,
				deps,
				resource.Config{ConvertedAttributes: &Config{}},
				logger,
			)
			test.That(t, err, test.ShouldBeNil)

			goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1.32 * 1000, Y: 0})
			success, err := ms.MoveOnMap(ctx, injectBase.Name(), goal, injectSlam.Name(), nil)
			testIfStoppable(t, success, err)
		})
	})
}
