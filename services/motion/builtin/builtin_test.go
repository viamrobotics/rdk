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
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
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
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/builtin/state"
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
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
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

func createMoveOnGlobeEnvironment(ctx context.Context, t *testing.T, origin *geo.Point, noise spatialmath.Pose, sleepTime int) (
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
		spatialmath.NewPoseFromPoint(r3.Vector{X: -10, Y: 0, Z: 0}),
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
		sleepTime,
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
		r3.Vector{X: 1, Y: 1, Z: 1}, "camera",
	)
	test.That(t, err, test.ShouldBeNil)

	injectedCamera := inject.NewCamera("injectedCamera")
	cameraLink := referenceframe.NewLinkInFrame(
		baseLink.Name(),
		spatialmath.NewPoseFromPoint(r3.Vector{X: 1, Y: 0, Z: 0}),
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

func createMoveOnMapEnvironment(
	ctx context.Context,
	t *testing.T,
	pcdPath string,
	geomSize float64,
) (kinematicbase.KinematicBase, motion.Service) {
	injectSlam := createInjectedSlam("test_slam", pcdPath)

	baseLink := createBaseLink(t)

	cfg := resource.Config{
		Name:  "test-base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: geomSize}},
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
		50,
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
			"when executeResponse.Replan is false & ReplanReason is empty and error is not nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: an error}",
			moveResponse{err: errors.New("an error")},
		},
		{
			"when executeResponse.Replan is true & ReplanReason is not empty and error is not nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: an error}",
			moveResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}, err: errors.New("an error")},
		},
		{
			"when executeResponse.Replan is true & ReplanReason is not empty and error is nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: <nil>}",
			moveResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}},
		},
		{
			"when executeResponse.Replan is false & ReplanReason is empty and error is nil",
			"builtin.moveResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: <nil>}",
			moveResponse{},
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
			"when replan is true and reason is non empty and error is nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: <nil>}",
			replanResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}},
		},
		{
			"when replan is true and reason is non empty and error is not nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:true, ReplanReason:\"some reason\"}, err: an error}",
			replanResponse{executeResponse: state.ExecuteResponse{Replan: true, ReplanReason: "some reason"}, err: errors.New("an error")},
		},
		{
			"when replan is false and error is nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: <nil>}",
			replanResponse{},
		},
		{
			"when replan is false and error is not nil",
			"builtin.replanResponse{executeResponse: state.ExecuteResponse{Replan:false, ReplanReason:\"\"}, err: an error}",
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
		grabPose := referenceframe.NewPoseInFrame("fakeCamera", spatialmath.NewPoseFromPoint(r3.Vector{X: 10.0, Y: 10.0, Z: 10.0}))
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
		grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, gripper.Named("pieceGripper"), grabPose, nil, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when mobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceArm", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
		_, err = ms.Move(ctx, arm.Named("pieceArm"), grabPose, nil, nil, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("succeeds when immobile component can be solved for destinations in own frame", func(t *testing.T) {
		ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
		defer teardown()
		grabPose := referenceframe.NewPoseInFrame("pieceGripper", spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -30, Z: -50}))
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
		grabPose := referenceframe.NewPoseInFrame("testFrame2", spatialmath.NewPoseFromPoint(r3.Vector{X: -20, Y: -130, Z: -40}))
		_, err = ms.Move(context.Background(), gripper.Named("pieceGripper"), grabPose, worldState, nil, nil)
		test.That(t, err, test.ShouldBeNil)
	})
}

func TestMoveWithObstacles(t *testing.T) {
	ms, teardown := setupMotionServiceFromConfig(t, "../data/moving_arm.json")
	defer teardown()

	t.Run("check a movement that should not succeed due to obstacles", func(t *testing.T) {
		testPose1 := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 370})
		testPose2 := spatialmath.NewPoseFromPoint(r3.Vector{X: 300, Y: 300, Z: -3500})
		_ = testPose2
		grabPose := referenceframe.NewPoseInFrame("world", spatialmath.NewPoseFromPoint(r3.Vector{X: -600, Y: -400, Z: 460}))
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
		_, ms := createMoveOnMapEnvironment(
			ctx,
			t,
			"slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd",
			110,
		)
		defer ms.Close(ctx)
		extra := make(map[string]interface{})
		req := motion.MoveOnMapReq{
			ComponentName: base.Named("test-base"),
			Destination:   goal,
			SlamName:      slam.Named("test_slam"),
			Extra:         extra,
		}
		mr, err := ms.(*builtIn).newMoveOnMapRequest(context.Background(), req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, mr, test.ShouldNotBeNil)
	})
}

func TestMoveOnMapPlans(t *testing.T) {
	ctx := context.Background()
	// goal x-position of 1.32m is scaled to be in mm
	// Orientation theta should be at least 3 degrees away from an integer multiple of 22.5 to ensure the position-only test functions.
	goalInBaseFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})
	goalInSLAMFrame := spatialmath.PoseBetweenInverse(motion.SLAMOrientationAdjustment, goalInBaseFrame)
	extra := map[string]interface{}{"smooth_iter": 5}
	extraPosOnly := map[string]interface{}{"smooth_iter": 5, "motion_profile": "position_only"}

	t.Run("ensure success of movement around obstacle", func(t *testing.T) {
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40)
		defer ms.Close(ctx)
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
		test.That(t, spatialmath.PoseAlmostEqualEps(endPos.Pose(), goalInBaseFrame, 15), test.ShouldBeTrue)
	})

	t.Run("check that straight line path executes", func(t *testing.T) {
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40)
		defer ms.Close(ctx)
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
		// TODO(RSDK-5758): unskip this
		t.Skip()
		kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40)
		defer ms.Close(ctx)
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

		test.That(t, spatialmath.PoseAlmostCoincidentEps(endPos.Pose(), goalInBaseFrame, 15), test.ShouldBeTrue)
		// Position only mode should not yield the goal orientation.
		test.That(t, spatialmath.OrientationAlmostEqualEps(
			endPos.Pose().Orientation(),
			goalInBaseFrame.Orientation(),
			0.05), test.ShouldBeFalse)
	})
}

func TestMoveOnMapSubsequent(t *testing.T) {
	ctx := context.Background()
	// goal x-position of 1.32m is scaled to be in mm
	goal1SLAMFrame := spatialmath.NewPose(r3.Vector{X: 1.32 * 1000, Y: 0}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 55})
	goal1BaseFrame := spatialmath.Compose(goal1SLAMFrame, motion.SLAMOrientationAdjustment)
	goal2SLAMFrame := spatialmath.NewPose(r3.Vector{X: 277, Y: 593}, &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 150})
	goal2BaseFrame := spatialmath.Compose(goal2SLAMFrame, motion.SLAMOrientationAdjustment)

	kb, ms := createMoveOnMapEnvironment(ctx, t, "pointcloud/octagonspace.pcd", 40)
	defer ms.Close(ctx)
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
		err := json.Unmarshal([]byte(entry1GoalLine), posepb)
		test.That(t, err, test.ShouldBeNil)

		return spatialmath.NewPoseFromProtobuf(posepb)
	}
	goalPose1 := logLineToGoalPose(goalLogsObserver[0].Entry.Message)
	test.That(t, spatialmath.PoseAlmostEqualEps(goalPose1, goal1BaseFrame, 10), test.ShouldBeTrue)
	goalPose2 := logLineToGoalPose(goalLogsObserver[1].Entry.Message)
	// This is the important test.
	test.That(t, spatialmath.PoseAlmostEqualEps(goalPose2, spatialmath.PoseBetween(goal1BaseFrame, goal2BaseFrame), 10), test.ShouldBeTrue)
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
	defer ms.Close(context.Background())

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

func TestPositionalReplanning(t *testing.T) {
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
		// TODO(RSDK-5634): this should be uncommented when this bug is fixed
		// {
		// 	// This also checks that `replan` is called under default conditions when "max_replans" is not set
		// 	name:            "check we fail to replan with a low cost factor",
		// 	noise:           r3.Vector{Y: epsilonMM + 0.1},
		// 	expectedErr:     "unable to create a new plan within replanCostFactor from the original",
		// 	expectedSuccess: false,
		// 	extra:           map[string]interface{}{"replan_cost_factor": 0.01, "smooth_iter": 5},
		// },
		{
			name:            "check we replan with a noisy sensor",
			noise:           r3.Vector{Y: epsilonMM + 0.1},
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d: plan failed", 4),
			expectedSuccess: false,
			extra:           map[string]interface{}{"replan_cost_factor": 10.0, "max_replans": 4, "smooth_iter": 5},
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, spatialmath.NewPoseFromPoint(tc.noise), 5)
		defer ms.Close(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      kb.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			MotionCfg:          motionCfg,
			Extra:              tc.extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Minute*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})

		if tc.expectedSuccess {
			test.That(t, err, test.ShouldBeNil)
		} else {
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

func TestObstacleReplanning(t *testing.T) {
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
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: -1000, Y: -1000, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 10, Y: 10, Z: 10}, "test-case-2")
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
				obstaclePosition := spatialmath.NewPoseFromPoint(r3.Vector{X: 1100, Y: 0, Z: 0})
				box, err := spatialmath.NewBox(obstaclePosition, r3.Vector{X: 100, Y: 100, Z: 10}, "test-case-1")
				test.That(t, err, test.ShouldBeNil)

				detection, err := viz.NewObjectWithLabel(pointcloud.New(), "test-case-1-detection", box.ToProtobuf())
				test.That(t, err, test.ShouldBeNil)

				return []*viz.Object{detection}, nil
			},
			expectedSuccess: false,
			expectedErr:     fmt.Sprintf("exceeded maximum number of replans: %d: plan failed", 0),
		},
	}

	testFn := func(t *testing.T, tc testCase) {
		t.Helper()
		injectedMovementSensor, _, kb, ms := createMoveOnGlobeEnvironment(
			ctx,
			t,
			gpsOrigin,
			spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			5000,
		)
		defer ms.Close(ctx)

		srvc, ok := ms.(*builtIn).visionServices[cfg.ObstacleDetectors[0].VisionServiceName].(*inject.VisionService)
		test.That(t, ok, test.ShouldBeTrue)
		srvc.GetObjectPointCloudsFunc = tc.getPCfunc

		req := motion.MoveOnGlobeReq{
			ComponentName:      kb.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			MotionCfg:          cfg,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Minute*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})

		if tc.expectedSuccess {
			test.That(t, err, test.ShouldBeNil)
		} else {
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

func TestMultiplePieces(t *testing.T) {
	var err error
	ms, teardown := setupMotionServiceFromConfig(t, "../data/fake_tomato.json")
	defer teardown()
	grabPose := referenceframe.NewPoseInFrame("c", spatialmath.NewPoseFromPoint(r3.Vector{X: -0, Y: -30, Z: -50}))
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
	testIfStoppable := func(t *testing.T, success bool, err, expectedErr error) {
		t.Helper()
		test.That(t, err, test.ShouldBeError, expectedErr)
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
		defer ms.Close(context.Background())

		t.Run("stop during Move(...) call", func(t *testing.T) {
			calledStopFunc = false
			success, err := ms.Move(ctx, injectArmName, goal, nil, nil, extra)
			testIfStoppable(t, success, err, failToReachGoalError)
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
				spatialmath.NewPoseFromPoint(r3.Vector{X: -10, Y: 0, Z: 0}),
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
			defer ms.Close(context.Background())

			ms.(*builtIn).fsService = fsSvc

			goal := geo.NewPoint(gpsPoint.Lat()+1e-4, gpsPoint.Lng()+1e-4)
			motionCfg := motion.MotionConfiguration{
				PlanDeviationMM:       10000,
				LinearMPerSec:         10,
				PositionPollingFreqHz: 4,
				ObstaclePollingFreqHz: 1,
			}

			req := motion.MoveOnGlobeReq{
				ComponentName:      injectBase.Name(),
				Destination:        goal,
				MovementSensorName: injectMovementSensor.Name(),
				MotionCfg:          &motionCfg,
				Extra:              extra,
			}
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeNil)

			timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
			defer timeoutFn()
			err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
				ComponentName: req.ComponentName,
				ExecutionID:   executionID,
				LastPlanOnly:  true,
			})

			expectedErr := errors.Wrap(errors.New("plan failed"), failToReachGoalError.Error())
			testIfStoppable(t, false, err, expectedErr)
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
			defer ms.Close(context.Background())

			fsSvc, err := createFrameSystemService(ctx, deps, fsParts, logger)
			test.That(t, err, test.ShouldBeNil)
			ms.(*builtIn).fsService = fsSvc

			goal := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 500})
			success, err := ms.MoveOnMap(ctx, injectBase.Name(), goal, injectSlam.Name(), extra)
			testIfStoppable(t, success, err, failToReachGoalError)
		})
	})
}

func TestMoveOnGlobe(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	// Near antarctica 🐧
	gpsPoint := geo.NewPoint(-70, 40)
	dst := geo.NewPoint(gpsPoint.Lat(), gpsPoint.Lng()+1e-5)
	expectedDst := r3.Vector{X: 380, Y: 0, Z: 0} // Relative pose to the starting point of the base; facing north, Y = forwards
	epsilonMM := 15.
	// create motion config
	extra := map[string]interface{}{
		"motion_profile": "position_only",
		"timeout":        5.,
		"smooth_iter":    5.,
	}

	t.Run("Changes to executions show up in PlanHistory", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        dst,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotBeEmpty)

		// returns the execution just created in the history
		ph, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph), test.ShouldEqual, 1)
		test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 1)
		test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph[0].Plan.Steps), test.ShouldNotEqual, 0)

		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: fakeBase.Name()})
		test.That(t, err, test.ShouldBeNil)

		ph2, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(ph2), test.ShouldEqual, 1)
		test.That(t, ph2[0].Plan.ExecutionID, test.ShouldResemble, executionID)
		test.That(t, len(ph2[0].StatusHistory), test.ShouldEqual, 2)
		test.That(t, ph2[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		test.That(t, ph2[0].StatusHistory[1].State, test.ShouldEqual, motion.PlanStateInProgress)
		test.That(t, len(ph2[0].Plan.Steps), test.ShouldNotEqual, 0)

		// Proves that calling StopPlan after the plan has reached a terminal state is idempotent
		err = ms.StopPlan(ctx, motion.StopPlanReq{ComponentName: fakeBase.Name()})
		test.That(t, err, test.ShouldBeNil)
		ph3, err := ms.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: req.ComponentName})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, ph3, test.ShouldResemble, ph2)
	})

	t.Run("returns error when called with an unknown component", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      base.Named("non existent base"),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        geo.NewPoint(0, 0),
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:base/non existent base\" not found"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns error when called with an unknown movement sensor", func(t *testing.T) {
		t.Parallel()
		_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: movementsensor.Named("non existent movement sensor"),
			Destination:        geo.NewPoint(0, 0),
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		e := "Resource missing from dependencies. Resource: rdk:component:movement_sensor/non existent movement sensor"
		test.That(t, err, test.ShouldBeError, errors.New(e))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns error when request would require moving more than 5 km", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        geo.NewPoint(0, 0),
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("cannot move more than 5 kilometers"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns error when destination is nil", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("destination cannot be nil"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns error when destination contains NaN", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
		}

		dests := []*geo.Point{
			geo.NewPoint(math.NaN(), math.NaN()),
			geo.NewPoint(0, math.NaN()),
			geo.NewPoint(math.NaN(), 0),
		}

		for _, d := range dests {
			req.Destination = d
			executionID, err := ms.MoveOnGlobe(ctx, req)
			test.That(t, err, test.ShouldBeError, errors.New("destination may not contain NaN"))
			test.That(t, executionID, test.ShouldResemble, uuid.Nil)
		}
	})

	t.Run("is able to reach a nearby geo point with empty values", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested NaN heading", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            math.NaN(),
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested positive heading", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point with a requested negative heading", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            -10000000,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("returns an error if the base provided is not a base", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      injectedMovementSensor.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("resource \"rdk:component:movement_sensor/test-gps\" not found"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error if the movement_sensor provided is not a movement_sensor", func(t *testing.T) {
		t.Parallel()
		_, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: fakeBase.Name(),
			Heading:            90,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("Resource missing from dependencies. Resource: rdk:component:base/test-base"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration is empty", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{},
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("is able to reach a nearby geo point when the motion configuration nil", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			Extra:              extra,
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)
	})

	t.Run("errors when motion configuration has a negative PlanDeviationMM", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{PlanDeviationMM: -1},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be negative"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("errors when motion configuration has a NaN PlanDeviationMM", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{PlanDeviationMM: math.NaN()},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("PlanDeviationMM may not be NaN"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when the motion configuration has a negative ObstaclePollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{ObstaclePollingFreqHz: -1},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be negative"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when the motion configuration has a NaN ObstaclePollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{ObstaclePollingFreqHz: math.NaN()},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("ObstaclePollingFreqHz may not be NaN"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when the motion configuration has a negative PositionPollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{PositionPollingFreqHz: -1},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be negative"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when the motion configuration has a NaN PositionPollingFreqHz", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{PositionPollingFreqHz: math.NaN()},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("PositionPollingFreqHz may not be NaN"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when motion configuration has a negative AngularDegsPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{AngularDegsPerSec: -1},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be negative"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when motion configuration has a NaN AngularDegsPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{AngularDegsPerSec: math.NaN()},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("AngularDegsPerSec may not be NaN"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when motion configuration has a negative LinearMPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{LinearMPerSec: -1},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be negative"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("returns an error when motion configuration has a NaN LinearMPerSec", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			MovementSensorName: injectedMovementSensor.Name(),
			Heading:            90,
			Destination:        dst,
			MotionCfg:          &motion.MotionConfiguration{LinearMPerSec: math.NaN()},
		}
		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeError, errors.New("LinearMPerSec may not be NaN"))
		test.That(t, executionID, test.ShouldResemble, uuid.Nil)
	})

	t.Run("ensure success to a nearby geo point", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}
		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{},
			MotionCfg:          motionCfg,
			Extra:              extra,
		}
		mr, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)

		test.That(t, mr.planRequest.Goal.Pose().Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, mr.planRequest.Goal.Pose().Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)

		planResp, err := mr.Plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(planResp.Waypoints), test.ShouldBeGreaterThan, 2)

		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("go around an obstacle", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		motionCfg := &motion.MotionConfiguration{PositionPollingFreqHz: 4, ObstaclePollingFreqHz: 1, PlanDeviationMM: epsilonMM}

		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
		boxDims := r3.Vector{X: 5, Y: 50, Z: 10}
		geometries, err := spatialmath.NewBox(boxPose, boxDims, "wall")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometries})
		startPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{geoObstacle},
			MotionCfg:          motionCfg,
			Extra:              extra,
		}
		mr, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		planResp, err := mr.Plan(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(planResp.Waypoints), test.ShouldBeGreaterThan, 2)

		executionID, err := ms.MoveOnGlobe(ctx, req)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, executionID, test.ShouldNotResemble, uuid.Nil)

		timeoutCtx, timeoutFn := context.WithTimeout(ctx, time.Second*15)
		defer timeoutFn()
		err = motion.PollHistoryUntilSuccessOrError(timeoutCtx, ms, time.Millisecond*5, motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})
		test.That(t, err, test.ShouldBeNil)

		endPose, err := fakeBase.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		movedPose := spatialmath.PoseBetween(startPose.Pose(), endPose.Pose())
		test.That(t, movedPose.Point().X, test.ShouldAlmostEqual, expectedDst.X, epsilonMM)
		test.That(t, movedPose.Point().Y, test.ShouldAlmostEqual, expectedDst.Y, epsilonMM)
	})

	t.Run("fail because of obstacle", func(t *testing.T) {
		t.Parallel()
		injectedMovementSensor, _, fakeBase, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)

		// Construct a set of obstacles that entirely enclose the goal point
		boxPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 50, Y: 0, Z: 0})
		boxDims := r3.Vector{X: 2, Y: 6660, Z: 10}
		geometry1, err := spatialmath.NewBox(boxPose, boxDims, "wall1")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 5000, Y: 0, Z: 0})
		boxDims = r3.Vector{X: 2, Y: 6660, Z: 10}
		geometry2, err := spatialmath.NewBox(boxPose, boxDims, "wall2")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 2500, Y: 2500, Z: 0})
		boxDims = r3.Vector{X: 6660, Y: 2, Z: 10}
		geometry3, err := spatialmath.NewBox(boxPose, boxDims, "wall3")
		test.That(t, err, test.ShouldBeNil)
		boxPose = spatialmath.NewPoseFromPoint(r3.Vector{X: 2500, Y: -2500, Z: 0})
		boxDims = r3.Vector{X: 6660, Y: 2, Z: 10}
		geometry4, err := spatialmath.NewBox(boxPose, boxDims, "wall4")
		test.That(t, err, test.ShouldBeNil)
		geoObstacle := spatialmath.NewGeoObstacle(gpsPoint, []spatialmath.Geometry{geometry1, geometry2, geometry3, geometry4})

		req := motion.MoveOnGlobeReq{
			ComponentName:      fakeBase.Name(),
			Destination:        dst,
			MovementSensorName: injectedMovementSensor.Name(),
			Obstacles:          []*spatialmath.GeoObstacle{geoObstacle},
			MotionCfg:          &motion.MotionConfiguration{},
			Extra:              extra,
		}
		moveRequest, err := ms.(*builtIn).newMoveOnGlobeRequest(ctx, req, nil, 0)
		test.That(t, err, test.ShouldBeNil)
		planResp, err := moveRequest.Plan(ctx)
		test.That(t, err, test.ShouldBeError)
		test.That(t, len(planResp.Motionplan), test.ShouldEqual, 0)
	})

	t.Run("check offset constructed correctly", func(t *testing.T) {
		t.Parallel()
		_, fsSvc, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
		defer ms.Close(ctx)
		baseOrigin := referenceframe.NewPoseInFrame("test-base", spatialmath.NewZeroPose())
		movementSensorToBase, err := fsSvc.TransformPose(ctx, baseOrigin, "test-gps", nil)
		if err != nil {
			movementSensorToBase = baseOrigin
		}
		test.That(t, movementSensorToBase.Pose().Point(), test.ShouldResemble, r3.Vector{X: 10, Y: 0, Z: 0})
	})
}

func TestMoveOnMapNew(t *testing.T) {
	ctx := context.Background()

	base, ms := createMoveOnMapEnvironment(
		context.Background(),
		t,
		"slam/example_cartographer_outputs/viam-office-02-22-3/pointcloud/pointcloud_4.pcd",
		110,
	)
	defer ms.Close(ctx)

	req := motion.MoveOnMapReq{
		ComponentName: base.Name(),
		Destination:   spatialmath.NewZeroPose(),
		SlamName:      slam.Named("test_slam"),
	}

	executionID, err := ms.MoveOnMapNew(ctx, req)
	test.That(t, err.Error(), test.ShouldEqual, "unimplemented")
	test.That(t, executionID, test.ShouldResemble, uuid.Nil)
}

func TestStopPlan(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)

	req := motion.StopPlanReq{}
	err := ms.StopPlan(ctx, req)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(req.ComponentName))
}

func TestListPlanStatuses(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)

	req := motion.ListPlanStatusesReq{}
	// returns no results as no move on globe calls have been made
	planStatusesWithIDs, err := ms.ListPlanStatuses(ctx, req)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(planStatusesWithIDs), test.ShouldEqual, 0)
}

func TestPlanHistory(t *testing.T) {
	ctx := context.Background()
	gpsPoint := geo.NewPoint(0, 0)
	//nolint:dogsled
	_, _, _, ms := createMoveOnGlobeEnvironment(ctx, t, gpsPoint, nil, 5)
	defer ms.Close(ctx)
	req := motion.PlanHistoryReq{}
	history, err := ms.PlanHistory(ctx, req)
	test.That(t, err, test.ShouldResemble, resource.NewNotFoundError(req.ComponentName))
	test.That(t, history, test.ShouldBeNil)
}

func TestNewValidatedMotionCfg(t *testing.T) {
	t.Run("returns expected defaults when given nil cfg", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     defaultAngularDegsPerSec,
			linearMPerSec:         defaultLinearMPerSec,
			obstaclePollingFreqHz: defaultObstaclePollingHz,
			positionPollingFreqHz: defaultPositionPollingHz,
			planDeviationMM:       defaultPlanDeviationM * 1e3,
			obstacleDetectors:     []motion.ObstacleDetectorName{},
		})
	})

	t.Run("returns expected defaults when given zero cfg", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(&motion.MotionConfiguration{})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     defaultAngularDegsPerSec,
			linearMPerSec:         defaultLinearMPerSec,
			obstaclePollingFreqHz: defaultObstaclePollingHz,
			positionPollingFreqHz: defaultPositionPollingHz,
			planDeviationMM:       defaultPlanDeviationM * 1e3,
			obstacleDetectors:     []motion.ObstacleDetectorName{},
		})
	})

	t.Run("allows overriding defaults", func(t *testing.T) {
		vmc, err := newValidatedMotionCfg(&motion.MotionConfiguration{
			AngularDegsPerSec:     10.,
			LinearMPerSec:         20.,
			PlanDeviationMM:       30.,
			PositionPollingFreqHz: 40,
			ObstaclePollingFreqHz: 50.,
			ObstacleDetectors: []motion.ObstacleDetectorName{
				{
					VisionServiceName: vision.Named("fakeVision"),
					CameraName:        camera.Named("fakeCamera"),
				},
			},
		})
		test.That(t, err, test.ShouldBeNil)
		test.That(t, vmc, test.ShouldResemble, &validatedMotionConfiguration{
			angularDegsPerSec:     10.,
			linearMPerSec:         20.,
			planDeviationMM:       30.,
			positionPollingFreqHz: 40.,
			obstaclePollingFreqHz: 50.,
			obstacleDetectors: []motion.ObstacleDetectorName{
				{
					VisionServiceName: vision.Named("fakeVision"),
					CameraName:        camera.Named("fakeCamera"),
				},
			},
		})
	})
}
