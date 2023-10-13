package explore

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	vSvc "go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
	"go.viam.com/rdk/vision"
)

var (
	testBaseName         = resource.NewName(base.API, "test_base")
	testFrameServiceName = resource.NewName(framesystem.API, "test_fs")
)

func createNewExploreMotionService(t *testing.T, ctx context.Context, logger golog.Logger, fakeBase base.Base, cam camera.Camera) (motion.Service, error) {

	var fsParts []*referenceframe.FrameSystemPart
	deps := make(resource.Dependencies)

	// create base link
	baseLink := createBaseLink(t, testBaseName.Name)
	fsParts = append(fsParts, &referenceframe.FrameSystemPart{FrameConfig: baseLink})
	deps[testBaseName] = fakeBase

	// create camera link
	if cam != nil {
		cameraLink := createBaseLink(t, cam.Name().Name)
		fsParts = append(fsParts, &referenceframe.FrameSystemPart{FrameConfig: cameraLink})
		deps[cam.Name()] = cam
	}

	// create frame service
	fs, err := createFrameSystemService(ctx, deps, fsParts, logger)
	if err != nil {
		return nil, err
	}
	deps[testFrameServiceName] = fs

	// create explore motion service
	exploreMotionConf := resource.Config{ConvertedAttributes: &Config{}}
	return NewExplore(ctx, deps, exploreMotionConf, logger)
}

func createMotionPlanToDest(ctx context.Context, ms *explore, dest spatialmath.Pose) ([][]referenceframe.Input, kinematicbase.KinematicBase, error) {
	extra := map[string]interface{}{
		"angular_degs_per_sec":          .25,
		"linear_m_per_sec":              .1,
		"obstacle_polling_frequency_hz": 1,
	}

	worldState, err := referenceframe.NewWorldState(nil, nil)
	if err != nil {
		return nil, nil, err
	}

	return ms.planMove(ctx, resource.NewName(base.API, "test_base"), dest, worldState, extra)
}

func TestExplorePlanMove(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	ms, err := createNewExploreMotionService(t, ctx, logger, fakeBase, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)

	msStruct := ms.(*explore)
	dest := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0})

	planInputs, kb, err := createMotionPlanToDest(ctx, msStruct, dest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
	test.That(t, len(planInputs), test.ShouldBeGreaterThan, 0)
	// Have plan
}

func TestUpdatingWorldState(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	mockCam := createMockCamera(ctx, true, r3.Vector{X: 0, Y: 10, Z: 0}, 100, nil)
	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)
	ms, err := createNewExploreMotionService(t, ctx, logger, fakeBase, mockCam)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)

	msStruct := ms.(*explore)
	dest := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0})

	planInputs, kb, err := createMotionPlanToDest(ctx, msStruct, dest)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
	test.That(t, len(planInputs), test.ShouldBeGreaterThan, 0)

	msStruct.kb = &kb
	msStruct.camera = mockCam
	msStruct.visionService = createMockVisionService(ctx, msStruct.camera, nil)

	worldState, err := msStruct.updateWorldState(ctx)
	test.That(t, err, test.ShouldBeNil)
	fmt.Printf("WorldState: %v\n", worldState)
	fmt.Println(worldState.Transforms())
	fs, err := msStruct.fsService.FrameSystem(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	fmt.Printf("Frame System: %v\n", fs)
	fmt.Printf("Frame System Names: %v\n", fs.FrameNames())
	obstacles, err := worldState.ObstaclesInWorldFrame(fs, nil)
	test.That(t, err, test.ShouldBeNil)
	for _, geo := range obstacles.Geometries() {
		fmt.Println(geo)
		fmt.Println(geo.Pose().Point())
	}
	fmt.Printf("Obstacles: %v\n", obstacles)

}

func createFakeBase(ctx context.Context, logger golog.Logger) (base.Base, error) {
	fakeBaseCfg := resource.Config{
		Name:  testBaseName.Name,
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 20}},
	}
	return baseFake.NewBase(ctx, nil, fakeBaseCfg, logger)
}

func createMockCamera(ctx context.Context, isObstacle bool, obstaclePose r3.Vector, obstacleProb int, expectedError error) camera.Camera {
	mockCamera := &inject.Camera{}

	mockCamera.NextPointCloudFunc = func(ctx context.Context) (pointcloud.PointCloud, error) {
		pc := pointcloud.New()
		err := pc.Set(obstaclePose, pointcloud.NewValueData(obstacleProb))
		if err != nil {
			return nil, expectedError
		}
		return pc, expectedError
	}
	return mockCamera
}

func createMockVisionService(ctx context.Context, cam camera.Camera, expectedError error) vSvc.Service {
	mockVisionService := &inject.VisionService{}

	mockVisionService.GetObjectPointCloudsFunc = func(ctx context.Context, cameraName string, extra map[string]interface{}) ([]*vision.Object, error) {
		pc, err := cam.NextPointCloud(ctx)
		if err != nil {
			return nil, expectedError
		}
		vObj, err := vision.NewObject(pc)
		if err != nil {
			return nil, expectedError
		}
		return []*vision.Object{vObj}, expectedError
	}
	return mockVisionService
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

func createBaseLink(t *testing.T, baseName string) *referenceframe.LinkInFrame {
	basePose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
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

func createCameraLink(t *testing.T, camName string) *referenceframe.LinkInFrame {
	camPose := spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0})
	camSphere, err := spatialmath.NewSphere(camPose, 10, "cam-sphere")
	test.That(t, err, test.ShouldBeNil)
	baseLink := referenceframe.NewLinkInFrame(
		referenceframe.World,
		spatialmath.NewZeroPose(),
		camName,
		camSphere,
	)
	return baseLink
}
