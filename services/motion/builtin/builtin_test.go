package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	viz "go.viam.com/rdk/vision"
)

func setupMotionServiceFromConfig(t *testing.T, configFilename string) (motion.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
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
	injectSlam.PointCloudMapFunc = func(ctx context.Context) (func() ([]byte, error), error) {
		return getPointCloudMap(filepath.Clean(artifact.MustPath(pcdPath)))
	}
	injectSlam.PositionFunc = func(ctx context.Context) (spatialmath.Pose, string, error) {
		return spatialmath.NewZeroPose(), "", nil
	}
	return injectSlam
}

func createBaseLink(t *testing.T) *referenceframe.LinkInFrame {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	baseSphere, err := spatialmath.NewSphere(basePose, 10, "base-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		"test-base",
		baseSphere,
	)
	return baseLink
}

func createFrameSystemService(
	ctx context.Context,
	deps resource.Dependencies,
	fsParts []*referenceframe.FrameSystemPart,
	logger logging.Logger,
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

func createMoveOnGlobeEnvironment(ctx context.Context, t *testing.T, origin *geo.Point, noise spatialmath.Pose) (
	*inject.MovementSensor, framesystem.Service, kinematicbase.KinematicBase, motion.Service,
) {
	logger := logging.NewTestLogger(t)

	// create fake base
	baseCfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	fakeBase, err := baseFake.NewBase(ctx, nil, baseCfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// create base link
	baseLink := createBaseLink(t)
	// create MovementSensor link
	movementSensorLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{-10, 0, 0}),
		"test-gps",
		nil,
	)

	// create a fake kinematic base
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	kinematicsOptions.PlanDeviationThresholdMM = 1 // can afford to do this for tests
	kb, err := kinematicbase.WrapWithFakePTGKinematics(
		ctx,
		fakeBase.(*baseFake.Base),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewZeroPose()),
		kinematicsOptions,
		noise,
	)
	test.That(t, err, test.ShouldBeNil)

	// create injected MovementSensor
	dynamicMovementSensor := inject.NewMovementSensor("test-gps")
	dynamicMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		poseInFrame, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		heading := poseInFrame.Pose().Orientation().OrientationVectorDegrees().Theta
		distance := poseInFrame.Pose().Point().Norm()
		pt := origin.PointAtDistanceAndBearing(distance*1e-6, heading)
		return pt, 0, nil
	}
	dynamicMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	dynamicMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	// create injected vision service
	injectedVisionSvc := inject.NewVisionService("injectedVisionSvc")

	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{1, 1, 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	injectedCamera := inject.NewCamera("injectedCamera")
	cameraLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{1, 0, 0}),
		"injectedCamera",
		cameraGeom,
	)

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: movementSensorLink},
		{FrameConfig: baseLink},
		{FrameConfig: cameraLink},
	}
	deps := resource.Dependencies{
		fakeBase.Name():              kb,
		dynamicMovementSensor.Name(): dynamicMovementSensor,
		injectedVisionSvc.Name():     injectedVisionSvc,
		injectedCamera.Name():        injectedCamera,
	}

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	return dynamicMovementSensor, fsSvc, kb, ms
}

func createMoveOnMapEnvironment(ctx context.Context, t *testing.T, pcdPath string) (kinematicbase.KinematicBase, motion.Service) {
	injectSlam := createInjectedSlam("test_slam", pcdPath)

	baseLink := createBaseLink(t)

	cfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 120}},
	}
	logger := logging.NewTestLogger(t)
	fakeBase, err := baseFake.NewBase(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	kinematicsOptions := kinematicbase.NewKinematicBaseOptions()
	kinematicsOptions.PlanDeviationThresholdMM = 1 // can afford to do this for tests
	kb, err := kinematicbase.WrapWithFakePTGKinematics(
		ctx,
		fakeBase.(*baseFake.Base),
		logger,
		referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewZeroPose()),
		kinematicsOptions,
		spatialmath.NewZeroPose(),
	)
	test.That(t, err, test.ShouldBeNil)

	deps := resource.Dependencies{injectSlam.Name(): injectSlam, fakeBase.Name(): kb}
	conf := resource.Config{ConvertedAttributes: &Config{}}

	// create the frame system
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: baseLink},
	}

	_, err = createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)

	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	return kb, ms
}

func TestMoveResponseString(t *testing.T) {
	type testCase struct {
		description  string
		expected     string
		moveResponse moveResponse
	}
	testCases := []testCase{
		{
			"when success is true and error is nil",
			"builtin.moveResponse{success: true, err: <nil>}",
			moveResponse{success: true},
		},
		{
			"when success is true and error is not nil",
			"builtin.moveResponse{success: true, err: an error}",
			moveResponse{success: true, err: errors.New("an error")},
		},
		{
			"when success is false and error is nil",
			"builtin.moveResponse{success: false, err: <nil>}",
			moveResponse{},
		},
		{
			"when success is false and error is not nil",
			"builtin.moveResponse{success: false, err: an error}",
			moveResponse{err: errors.New("an error")},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			test.That(t, tc.moveResponse.String(), test.ShouldEqual, tc.expected)
		})
	}
}

func TestReplanResponseString(t *testing.T) {
	type testCase struct {
		description    string
		expected       string
		replanResponse replanResponse
	}
	testCases := []testCase{
		{
			"when replan is true and error is nil",
			"builtin.replanResponse{replan: true, err: <nil>}",
			replanResponse{replan: true},
		},
		{
			"when replan is true and error is not nil",
			"builtin.replanResponse{replan: true, err: an error}",
			replanResponse{replan: true, err: errors.New("an error")},
		},
		{
			"when replan is false and error is nil",
			"builtin.replanResponse{replan: false, err: <nil>}",
			replanResponse{},
		},
		{
			"when replan is false and error is not nil",
			"builtin.replanResponse{replan: false, err: an error}",
			replanResponse{err: errors.New("an error")},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			test.That(t, tc.replanResponse.String(), test.ShouldEqual, tc.expected)
		})
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
	t.Skip()
	t.Parallel()
	ctx := context.Background()
	// goal position is scaled to be in mm
	goal := spatialmath.NewPoseFromPoint(r3.Vector{X: -32.508 * 1000, Y: -2.092 * 1000})

	t.Run("test tp-space planning on office map", func(t *testing.T) {
		t.Parallel()
		_, ms := createMoveOnMapEnvironment(ctx, t, "slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd")
		extra := make(map[string]interface{})
		valExtra, err := newValidatedExtra(extra)
		test.That(t, err, test.ShouldBeNil)
		mr, err := ms.(*builtIn).newMoveOnMapRequest(
			context.Background(),
			base.Named("test-base"),
			goal,
			slam.Named("test_slam"),
			valExtra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mr, test.ShouldNotBeNil)
	})
}

func TestMoveOnMapPlans(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// goal x-position of 1.32m is scaled to be in mm
	goalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 1.32 * 1000, Y: 0})
	goalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, goalInBaseFrame)
	extra := map[string]interface{}{"smooth_iter": 5}
	extraPosOnly := map[string]interface{}{"smooth_iter": 5, "motion_profile": "position_only"}

	t.Run("ensure success of movement around obstacle", func(t *testing.T) {
		t.Parallel()
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test-base"),
			goalInSLAMFrame,
			slam.Named("test_slam"),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
		endPos, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goalInBaseFrame, 10), test.ShouldBeTrue)
	})

	t.Run("check that straight line path executes", func(t *testing.T) {
		t.Parallel()
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		easyGoalInBaseFrame := spatialmath.NewPoseFromPoint(r3.Vector{X: 0.277 * 1000, Y: 0.593 * 1000})
		easyGoalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, easyGoalInBaseFrame)
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test-base"),
			easyGoalInSLAMFrame,
			slam.Named("test_slam"),
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
		endPos, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), easyGoalInBaseFrame, 10), test.ShouldBeTrue)
	})

	t.Run("check that position-only mode executes", func(t *testing.T) {
		t.Parallel()
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
		success, err := ms.MoveOnMap(
			context.Background(),
			base.Named("test-base"),
			goalInSLAMFrame,
			slam.Named("test_slam"),
			extraPosOnly,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
		endPos, err := kb.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, spatialmath.PoseAlmostCoincidentEps(endPos.Pose(), goalInBaseFrame, 10), test.ShouldBeTrue)
	})
}

func TestMoveOnMapSubsequent(t *testing.T) {
	ctx := context.Background()
	// goal x-position of 1.32m is scaled to be in mm
	goal1SLAMFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})
	goal1BaseFrame := spatialmath.Compose(goal1SLAMFrame, motion.SLAMOrientationAdjustment)
	goal2SLAMFrame := spatialmath.NewPose(r3.Vector{X: 277, Y: 593}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 150})
	goal2BaseFrame := spatialmath.Compose(goal2SLAMFrame, motion.SLAMOrientationAdjustment)

	kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd")
	msBuiltin, ok := ms.(*builtIn)
	test.That(t, ok, test.ShouldBeTrue)

	// Overwrite `logger` so we can use a handler to test for correct log messages
	logger, observer := logging.NewObservedTestLogger(t)
	msBuiltin.logger = logger

	extra := map[string]interface{}{"smooth_iter": 5}
	success, err := msBuiltin.MoveOnMap(
		context.Background(),
		base.Named("test-base"),
		goal1SLAMFrame,
		slam.Named("test_slam"),
		extra,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, success, test.ShouldNotBeNil)
	endPos, err := kb.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	logger.Debug(spatialmath.PoseToProtobuf(endPos.Pose()))
	test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goal1BaseFrame, 10), test.ShouldBeTrue)

	// Now, we try to go to the second goal. Since the `CurrentPosition` of our base is at `goal1`, the pose that motion solves for and
	// logs should be {x:-1043  y:593}
	success, err = msBuiltin.MoveOnMap(
		context.Background(),
		base.Named("test-base"),
		goal2SLAMFrame,
		slam.Named("test_slam"),
		extra,
	)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, success, test.ShouldNotBeNil)
	endPos, err = kb.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	logger.Debug(spatialmath.PoseToProtobuf(endPos.Pose()))
	test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goal2BaseFrame, 1), test.ShouldBeTrue)

	// We don't actually surface the internal motion planning goal; we report to the user in terms of what the user provided us.
	// Thus, we must do string surgery on the internal `motionplan` logs to extract the requested relative pose and check it is correct.
	goalLogsObserver := observer.FilterMessageSnippet("Goal: reference_frame:").All()
	test.That(t, len(goalLogsObserver), test.ShouldEqual, 2)
	logLineToGoalPose := func(logString string) spatialmath.Pose {
		entry1GoalLine := strings.Split(strings.Split(logString, "\n")[1], "pose:")[1]
		// logger formatting is weird and will use sometimes one, sometimes two spaces. strings.Replace can't handle variability, so regex.
		re := regexp.MustCompile(`\s+`)
		entry1GoalLineBytes := re.ReplaceAll([]byte(entry1GoalLine), []byte(",\""))
		entry1GoalLine = strings.ReplaceAll(string(entry1GoalLineBytes), "{", "{\"")
		entry1GoalLine = strings.ReplaceAll(entry1GoalLine, ":", "\":")

		posepb := &commonpb.Pose{}
		json.Unmarshal([]byte(entry1GoalLine), posepb)
		return spatialmath.NewPoseFromProtobuf(posepb)
	}
	goalPose1 := logLineToGoalPose(goalLogsObserver[0].Entry.Message)
	test.That(t, spatialmath.PoseAlmostEqualEps(goalPose1, goal1BaseFrame, 10), test.ShouldBeTrue)
	goalPose2 := logLineToGoalPose(goalLogsObserver[1].Entry.Message)
	// This is the important test.
	test.That(t, spatialmath.PoseAlmostEqualEps(goalPose2, spatialmath.PoseBetween(goal1BaseFrame, goal2BaseFrame), 1), test.ShouldBeTrue)
}

func TestMoveOnMapTimeout(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(ctx, "../data/real_wheeled_base.json", logger)
	test.That(t, err, test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() {
		test.That(t, myRobot.Close(context.Background()), test.ShouldBeNil)
	}()

	injectSlam := createInjectedSlam("test_slam", "pointcloud/octagonspace.pcd")

	realBase, err := base.FromRobot(myRobot, "test-base")
	test.That(t, err, test.ShouldBeNil)

	deps := resource.Dependencies{
		injectSlam.Name(): injectSlam,
		realBase.Name():   realBase,
	}
	fsParts := []*referenceframe.FrameSystemPart{
		{FrameConfig: createBaseLink(t)},
	}

	conf := resource.Config{ConvertedAttributes: &Config{}}
	ms, err := NewBuiltIn(ctx, deps, conf, logger)
	test.That(t, err, test.ShouldBeNil)

	fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
	test.That(t, err, test.ShouldBeNil)
	ms.(*builtIn).fsService = fsSvc

	easyGoal := spatialmath.NewPoseFromPoint(r3.Vector{X: 1001, Y: 1001})
	// create motion config
	motionCfg := make(map[string]interface{})
	motionCfg["timeout"] = 0.01
	success, err := ms.MoveOnMap(
		context.Background(),
		base.Named("test-base"),
		easyGoal,
		slam.Named("test_slam"),
		motionCfg,
	)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, success, test.ShouldBeFalse)
}

func TestMoveOnGlobe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Near antarctica 🐧
	gpsPoint := geo.NewPoint(-70, 40)

	// create motion config
	extra := make(map[string]interface{})
	extra["motion_profile"] = "position_only"
	extra["timeout"] = 5.
	extra["smooth_iter"] = 5.
	valExtra, err := newValidatedExtra(extra)
	test.That(t, err, test.ShouldBeNil)

	extraSmooth := map[string]interface{}{"smooth_iter": 5}

	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	expectedDst := r3.Vector{380, 0, 0} // Relative pose to the starting point of the base; facing north, Y = forwards
	epsilonMM := 15.

	t.Run("returns error when called with an unknown component", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			base.Named("non existent base"),
			geo.NewPoint(0, 0),
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:base/non existent base\" not found"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns error when called with an unknown movement sensor", func(t *testing.T) {
		t.Parallel()
		_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			geo.NewPoint(0, 0),
			0,
			movementsensor.Named("non existent movement sensor"),
			nil,
			nil,
			nil,
		)
		e := "\"rdk:component:movement_sensor/non existent movement sensor\" missing from dependencies"
		test.That(t, err, test.ShouldBeError, errors.New(e))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns error when request would require moving more than 5 km", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			geo.NewPoint(0, 0),
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("cannot move more than 5 kilometers"))
		test.That(t, success, test.ShouldBeFalse)
	})
	t.Run("returns error when destination is nil", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			nil,
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("destination cannot be nil"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns error when destination contains NaN", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			geo.NewPoint(math.NaN(), 0),
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("destination may not contain NaN"))
		test.That(t, success, test.ShouldBeFalse)

		success, err = ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			geo.NewPoint(0, math.NaN()),
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("destination may not contain NaN"))
		test.That(t, success, test.ShouldBeFalse)

		success, err = ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			geo.NewPoint(math.NaN(), math.NaN()),
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("destination may not contain NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("is able to reach a nearby geo point with empty values", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			0,
			injectedMovementSensor.Name(),
			nil,
			nil,
			extraSmooth,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("is able to reach a nearby geo point with a requested NaN heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			math.NaN(),
			injectedMovementSensor.Name(),
			nil,
			nil,
			extraSmooth,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("is able to reach a nearby geo point with a requested positive heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			10000000,
			injectedMovementSensor.Name(),
			nil,
			nil,
			extraSmooth,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("is able to reach a nearby geo point with a requested negative heading", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			-10000000,
			injectedMovementSensor.Name(),
			nil,
			nil,
			extraSmooth,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("returns an error if the base provided is not a base", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			injectedMovementSensor.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:movement_sensor/test-gps\" not found"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error if the movement_sensor provided is not a movement_sensor", func(t *testing.T) {
		t.Parallel()
		_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			fakeBase.Name(),
			nil,
			nil,
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("\"rdk:component:base/test-base\" missing from dependencies"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration is empty", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{},
			extraSmooth,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("errors when motion configuration has a negative PlanDeviationMM", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{PlanDeviationMM: -1},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be negative"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("errors when motion configuration has a NaN PlanDeviationMM", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{PlanDeviationMM: math.NaN()},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when the motion configuration has a negative ObstaclePollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{ObstaclePollingFreqHz: -1},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be negative"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when the motion configuration has a NaN ObstaclePollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{ObstaclePollingFreqHz: math.NaN()},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when the motion configuration has a negative PositionPollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{PositionPollingFreqHz: -1},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be negative"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when the motion configuration has a NaN PositionPollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{PositionPollingFreqHz: math.NaN()},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when motion configuration has a negative AngularDegsPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{AngularDegsPerSec: -1},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be negative"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when motion configuration has a NaN AngularDegsPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{AngularDegsPerSec: math.NaN()},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when motion configuration has a negative LinearMPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{LinearMPerSec: -1},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be negative"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("returns an error when motion configuration has a NaN LinearMPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		defer ms.Close(ctx)
		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			90,
			injectedMovementSensor.Name(),
			nil,
			&motion.MotionConfiguration{LinearMPerSec: math.NaN()},
			nil,
		)
		test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be NaN"))
		test.That(t, success, test.ShouldBeFalse)
	})

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}
		mr, err := ms.(*builtIn).newMoveOnGlobeRequest(
			ctx,
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{},
			motionCfg,
			nil,
			valExtra,
		)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, mr.planRequest.Goal.Pose().Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, mr.planRequest.Goal.Pose().Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)

		waypoints, err := mr.plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(waypoints), test.ShouldBeGreaterThan, 2)

		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			0,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{},
			motionCfg,
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}

		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{5, 50, 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})
		startPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		mr, err := ms.(*builtIn).newMoveOnGlobeRequest(
			ctx,
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			motionCfg,
			nil,
			valExtra,
		)
		test.That(t, err, test.ShouldBeNil)
		waypoints, err := mr.plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(waypoints), test.ShouldBeGreaterThan, 2)

		success, err := ms.MoveOnGlobe(
			ctx,
			fakeBase.Name(),
			dst,
			0,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			motionCfg,
			extra,
		)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, success, test.ShouldBeTrue)

		endPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		movedPose := spatialmath.PoseBetween(startPose.Pose(), endPose.Pose())
		test.That(t, movedPose.Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, movedPose.Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)
	})

	t.Run("fail because of obstacle", func(t *testing.T) {
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)

		// Construct a set of obstacles that entirely enclose the goal point
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
		boxDims := r3.Vector{2, 6660, 10}
		geometry1, err := spatialmath.NewBox(boxPose, boxDims, "wall1")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{5000, 0, 0})
		boxDims = r3.Vector{2, 6660, 10}
		geometry2, err := spatialmath.NewBox(boxPose, boxDims, "wall2")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{2500, 2500, 0})
		boxDims = r3.Vector{6660, 2, 10}
		geometry3, err := spatialmath.NewBox(boxPose, boxDims, "wall3")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{2500, -2500, 0})
		boxDims = r3.Vector{6660, 2, 10}
		geometry4, err := spatialmath.NewBox(boxPose, boxDims, "wall4")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometry1, geometry2, geometry3, geometry4})

		moveRequest, err := ms.(*builtIn).newMoveOnGlobeRequest(
			ctx,
			fakeBase.Name(),
			dst,
			injectedMovementSensor.Name(),
			[]*spatialmath.GeoObstacle{geoObstacle},
			&motion.MotionConfiguration{},
			nil,
			valExtra,
		)
		test.That(t, err, test.ShouldBeNil)
		plan, err := moveRequest.plan(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, len(plan), test.ShouldEqual, 0)
	})

	t.Run("check offset constructed correctly", func(t *testing.T) {
		t.Parallel()
		_, fsSvc, _, _ := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
		baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
		movementSensorToBase, err := fsSvc.TransformPose(ctx, baseOrigin, "test-gps", nil)
		if err != nil {
			movementSensorToBase = baseOrigin
		}
		test.That(t, movementSensorToBase.Pose().Point(), test.ShouldResemble, r3.Vector{10, 0, 0})
	})
}

func TestReplanning(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gpsPoint := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	epsilonMM := 15.
	motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 100, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}

	type testCase struct {
		name            string
		noise           r3.Vector
		expectedSuccess bool
		expectedErr     string
		extra           map[string]interface{}
	}

	testCases := []testCase{
		{
			name:            "check we dont replan with a good sensor",
			noise:           r3.Vector{Y: epsilonMM - 0.1},
			expectedSuccess: true,
			extra:           map[string]interface{}{"smooth_iter": 5},
		},
		{
			// This also checks that `replan` is called under default conditions when "max_replans" is not set
			name:            "check we fail to replan with a low cost factor",
			noise:           r3.Vector{Y: epsilonMM + 0.1},
			expectedErr:     "unable to create a new plan within replanCostFactor from the original",
			expectedSuccess: false,
			extra:           map[string]interface{}{"replan_cost_factor": 0.01, "smooth_iter": 5},
		},
		{
			name:            "check we replan with a noisy sensor",
			noise:           r3.Vector{Y: epsilonMM + 0.1},
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d", 4),
			expectedSuccess: false,
			extra:           map[string]interface{}{"replan_cost_factor": 10.0, "max_replans": 4, "smooth_iter": 5},
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, spatialmath.NewPoseFromPoint(tc.noise))

		success, err := ms.MoveOnGlobe(ctx, kb.Name(), dst, 0, injectedMovementSensor.Name(), nil, motionCfg, tc.extra)

		if tc.expectedSuccess {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, success, test.ShouldBeTrue)
		} else {
			test.That(t, success, test.ShouldBeFalse)
			test.That(t, err.Error(), test.ShouldEqual, tc.expectedErr)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testFn(t, c)
		})
	}
}

func TestObstacleDetection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gpsOrigin := geo.NewPoint(0, 0)
	dst := geo.NewPoint(gpsOrigin.Lat(), gpsOrigin.Lng()+1e-5)
	epsilonMM := 15.

	type testCase struct {
		name            string
		getPCfunc       func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error)
		expectedSuccess bool
		expectedErr     string
	}

	obstacleDetectorSlice := []motion.ObstacleDetectorName{
		{VisionServiceName: vision.Named("injectedVisionSvc"), CameraName: camera.Named("injectedCamera")},
	}

	cfg := &motion.MotionConfiguration{
		PositionPollingFreqHz: 1, ObstaclePollingFreqHz: 100, PlanDeviationMM: epsilonMM, ObstacleDetectors: obstacleDetectorSlice,
	}

	extra := map[string]interface{}{"max_replans": 0, "max_ik_solutions": 1, "smooth_iter": 1}

	testCases := []testCase{
		{
			name: "ensure no replan from discovered obstacles",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{-1000, -1000, 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{10, 10, 10}, "test-case-2")
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-2-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: true,
		},
		{
			name: "ensure replan due to obstacle collision",
			getPCfunc: func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*viz.Object, error) {
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{50, 0, 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{100, 100, 10}, "test-case-1")
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-1-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: false,
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d", 0),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(ctx, t, gpsOrigin, spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0}))

		srvc, ok := ms.(*builtIn).visionServices[cfg.ObstacleDetectors[0].VisionServiceName].(*inject.VisionService)
		test.That(t, ok, test.ShouldBeTrue)
		srvc.GetObjectPointCloudsFunc = tc.getPCfunc

		success, err := ms.MoveOnGlobe(ctx, kb.Name(), dst, 0, injectedMovementSensor.Name(), nil, cfg, extra)

		if tc.expectedSuccess {
			test.That(t, err, test.ShouldBeNil)
			test.That(t, success, test.ShouldBeTrue)
		} else {
			test.That(t, success, test.ShouldBeFalse)
			test.That(t, err.Error(), test.ShouldEqual, tc.expectedErr)
		}
	}

	for _, tc := range testCases {
		c := tc // needed to workaround loop variable not being captured by func literals
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testFn(t, c)
		})
	}
}

func TestCheckPlan(t *testing.T) {
	t.Skip() // TODO(RSDK-5404): fix flakiness
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	// orign as gps point
	originPoint := geo.NewPoint(-70, 40)

	destPoint := geo.NewPoint(originPoint.Lat(), originPoint.Lng()+1e-5)

	// create env
	injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, originPoint, nil)

	// create motion config
	extra := make(map[string]interface{})
	// fail if we don't find a plan in 15 seconds
	extra["timeout"] = 15.
	validatedExtra, err := newValidatedExtra(extra)
	test.That(t, err, test.ShouldBeNil)

	// get plan and kinematic base
	moveRequest, err := ms.(*builtIn).newMoveOnGlobeRequest(
		context.Background(),
		fakeBase.Name(),
		destPoint,
		injectedMovementSensor.Name(),
		nil,
		&motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: 15.},
		nil,
		validatedExtra,
	)
	test.That(t, err, test.ShouldBeNil)

	plan, err := motionplan.PlanMotion(ctx, moveRequest.planRequest)
	test.That(t, err, test.ShouldBeNil)

	// construct framesystem
	newFS := referenceframe.NewEmptyFrameSystem("test-fs")
	err = newFS.AddFrame(moveRequest.kinematicBase.Kinematics(), newFS.World())
	test.That(t, err, test.ShouldBeNil)

	startPose := spatialmath.NewPoseFromPoint(r3.Vector{0, 0, 0})
	errorState := startPose
	floatList := []float64{0, 0, 0}
	inputs := referenceframe.FloatsToInputs(floatList)

	t.Run("without obstacles - ensure success", func(t *testing.T) {
		err := motionplan.CheckPlan(moveRequest.kinematicBase.Kinematics(), plan, nil, newFS,
			startPose, inputs, errorState, lookAheadDistanceMM, logger)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("with a blocking obstacle - ensure failure", func(t *testing.T) {
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{380, 0, 0}), // Y means forwards from the base's pose at the start of the motion
			r3.Vector{10, 10, 10}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(moveRequest.kinematicBase.Kinematics(), plan, worldState, newFS,
			startPose, inputs, errorState, lookAheadDistanceMM, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})

	// create camera_origin frame
	cameraOriginFrame, err := referenceframe.NewStaticFrame("camera-origin", spatialmath.NewPoseFromPoint(r3.Vector{0, -30, 0}))
	test.That(t, err, test.ShouldBeNil)
	err = newFS.AddFrame(cameraOriginFrame, moveRequest.kinematicBase.Kinematics())
	test.That(t, err, test.ShouldBeNil)

	// create camera geometry
	cameraGeom, err := spatialmath.NewBox(
		spatialmath.NewZeroPose(),
		r3.Vector{1, 1, 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	// create cameraFrame and add to framesystem. Camera should be pointed forwards.
	cameraFrame, err := referenceframe.NewStaticFrameWithGeometry(
		"camera-frame", spatialmath.NewZeroPose(), cameraGeom,
	)
	test.That(t, err, test.ShouldBeNil)
	err = newFS.AddFrame(cameraFrame, cameraOriginFrame)
	test.That(t, err, test.ShouldBeNil)

	t.Run("ensure transforms of obstacles works - no collision", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{1500, -6, 0}),
			r3.Vector{10, 10, 10}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(moveRequest.kinematicBase.Kinematics(), plan, worldState, newFS,
			startPose, inputs, errorState, lookAheadDistanceMM, logger)
		test.That(t, err, test.ShouldBeNil)
	})
	t.Run("ensure transforms of obstacles works - collision with camera", func(t *testing.T) {
		// create obstacle
		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{400, 0, 0}),
			r3.Vector{50, 50, 10}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(cameraFrame.Name(), geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(moveRequest.kinematicBase.Kinematics(), plan, worldState, newFS,
			startPose, inputs, errorState, lookAheadDistanceMM, logger)
		test.That(t, err, test.ShouldNotBeNil)
	})
	t.Run("non nil error state - ensure success", func(t *testing.T) {
		errorState := spatialmath.NewPoseFromPoint(r3.Vector{0, 2600, 0})

		obstacle, err := spatialmath.NewBox(
			spatialmath.NewPoseFromPoint(r3.Vector{150, 0, 0}),
			r3.Vector{10, 10, 1}, "obstacle",
		)
		test.That(t, err, test.ShouldBeNil)
		geoms := []spatialmath.Geometry{obstacle}
		gifs := []*referenceframe.GeometriesInFrame{referenceframe.NewGeometriesInFrame(referenceframe.World, geoms)}

		worldState, err := referenceframe.NewWorldState(gifs, nil)
		test.That(t, err, test.ShouldBeNil)

		err = motionplan.CheckPlan(moveRequest.kinematicBase.Kinematics(), plan, worldState, newFS,
			startPose, inputs, errorState, lookAheadDistanceMM, logger)
		test.That(t, err, test.ShouldBeNil)
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
	logger := logging.NewTestLogger(t)
	failToReachGoalError := errors.New("failed to reach goal")
	calledStopFunc := false
	testIfStoppable := func(t *testing.T, success bool, err error) {
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err, test.ShouldEqual, failToReachGoalError)
		test.That(t, success, test.ShouldBeFalse)
		test.That(t, calledStopFunc, test.ShouldBeTrue)
	}
	extra := map[string]interface{}{"smooth_iter": 5}

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
			success, err := ms.Move(ctx, injectArmName, goal, nil, nil, extra)
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
		baseLink := createBaseLink(t)

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

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)

			conf := resource.Config{ConvertedAttributes: &Config{}}
			ms, err := NewBuiltIn(ctx, deps, conf, logger)
			test.That(t, err, test.ShouldBeNil)

			ms.(*builtIn).fsService = fsSvc

			goal := geo.NewPoint(gpsPoint.Lat()+1e-4, gpsPoint.Lng()+1e-4)
			motionCfg := motion.MotionConfiguration{
				PlanDeviationMM:       10000,
				LinearMPerSec:         10,
				PositionPollingFreqHz: 4,
				ObstaclePollingFreqHz: 1,
			}
			success, err := ms.MoveOnGlobe(
				ctx, injectBase.Name(), goal, 0, injectMovementSensor.Name(),
				nil, &motionCfg, extra,
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
			fsParts := []*referenceframe.FrameSystemPart{
				{FrameConfig: baseLink},
			}

			ms, err := NewBuiltIn(
				ctx,
				deps,
				resource.Config{ConvertedAttributes: &Config{}},
				logger,
			)
			test.That(t, err, test.ShouldBeNil)

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)
			ms.(*builtIn).fsService = fsSvc

			goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 500})
			success, err := ms.MoveOnMap(ctx, injectBase.Name(), goal, injectSlam.Name(), extra)
			testIfStoppable(t, success, err)
		})
	})
}

func TestMoveOnGlobeNew(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
	defer ms.Close(ctx)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)

	req := motion.MoveOnGlobeReq{
		ComponentName:      fakeBase.Name(),
		MovementSensorName: injectedMovementSensor.Name(),
		Destination:        dst,
	}
	executionID, err := ms.MoveOnGlobeNew(ctx, req)
	test.That(t, err, test.ShouldBeError, errUnimplemented)
	test.That(t, executionID, test.ShouldBeEmpty)
}

func TestStopPlan(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
	defer ms.Close(ctx)

	req := motion.StopPlanReq{ComponentName: fakeBase.Name()}
	err := ms.StopPlan(ctx, req)
	test.That(t, err, test.ShouldEqual, errUnimplemented)
}

func TestListPlanStatuses(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
	defer ms.Close(ctx)

	req := motion.ListPlanStatusesReq{}
	planStatusesWithIDs, err := ms.ListPlanStatuses(ctx, req)
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	test.That(t, planStatusesWithIDs, test.ShouldBeNil)
}

func TestPlanHistory(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil)
	defer ms.Close(ctx)

	req := motion.PlanHistoryReq{ComponentName: fakeBase.Name()}
	history, err := ms.PlanHistory(ctx, req)
	test.That(t, err, test.ShouldEqual, errUnimplemented)
	test.That(t, history, test.ShouldBeNil)
}
