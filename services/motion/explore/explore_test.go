package explore

import (
	"context"
	"fmt"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/base/kinematicbase"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

var (
	testBaseName         = resource.NewName(base.API, "test_base")
	testCameraName       = resource.NewName(camera.API, "test_camera")
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
		fmt.Println("cam.Name().Name: ", cam.Name().Name)
		cameraLink := createCameraLink(t, cam.Name().Name, fakeBase.Name().Name)
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
}

func TestUpdatingWorldState(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Create fake camera
	fakeCamera, err := createFakeCamera(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create fake base
	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create explore motion service
	ms, err := createNewExploreMotionService(t, ctx, logger, fakeBase, fakeCamera)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)

	msStruct := ms.(*explore)

	cases := []struct {
		description string
		destination spatialmath.Pose
		obstacle    obstacleMetadata
		expectedErr error
	}{
		{
			description: "1",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 50, Z: 0},
				data:     100,
				label:    "updatedState_obs",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			// Create motionplan plan
			planInputs, kb, err := createMotionPlanToDest(ctx, msStruct, tt.destination)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
			test.That(t, len(planInputs), test.ShouldBeGreaterThan, 0)

			msStruct.kb = &kb
			msStruct.camera = fakeCamera

			var plan motionplan.Plan
			for _, inputs := range planInputs {
				input := make(map[string][]referenceframe.Input)
				input[kb.Name().Name] = inputs
				plan = append(plan, input)
			}

			// Add vision service with obstacle and update the world state
			msStruct.visionService = createMockVisionService(ctx, tt.obstacle, nil)

			worldState, err := msStruct.updateWorldState(ctx)
			test.That(t, err, test.ShouldBeNil)

			//Confirm obstacles encompass point
			var noObstacle obstacleMetadata
			if tt.obstacle != noObstacle {
				fs, err := msStruct.fsService.FrameSystem(ctx, nil)
				test.That(t, err, test.ShouldBeNil)

				obstacles, err := worldState.ObstaclesInWorldFrame(fs, nil)
				test.That(t, err, test.ShouldBeNil)

				collisionDetected, err := geometriesContainsPoint(obstacles.Geometries(), tt.obstacle.position)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, collisionDetected, test.ShouldBeTrue)
			}

			// Run check of motionplan plan
			msStruct.backgroundWorkers.Add(1)
			goutils.ManagedGo(func() {
				msStruct.checkPartialPlan(ctx, plan, worldState)
			}, msStruct.backgroundWorkers.Done)

			// Check for response from obstacle channel
			resp := <-msStruct.obstacleChan
			test.That(t, resp.err, test.ShouldBeNil)
			test.That(t, resp.success, test.ShouldBeTrue)
		})
	}
}
