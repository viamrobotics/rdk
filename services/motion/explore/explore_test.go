package explore

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
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

	obstaclePosition := r3.Vector{X: 0, Y: 10, Z: 0}
	mockCam := createMockCamera(ctx, true, obstaclePosition, 100, nil)
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
		test.That(t, obstaclePosition, test.ShouldResemble, geo.Pose().Point())
	}
	fmt.Printf("Obstacles: %v\n", obstacles)

}
