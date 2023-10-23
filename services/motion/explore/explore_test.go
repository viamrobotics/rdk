package explore

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
)

var (
	testBaseName         = resource.NewName(base.API, "test_base")
	testCameraName1      = resource.NewName(camera.API, "test_camera1")
	testCameraName2      = resource.NewName(camera.API, "test_camera2")
	testFrameServiceName = resource.NewName(framesystem.API, "test_fs")
	defaultMotionCfg     = motion.MotionConfiguration{
		AngularDegsPerSec:     0.25,
		LinearMPerSec:         0.1,
		ObstaclePollingFreqHz: 2,
	}
)

type obstacleMetadata struct {
	position r3.Vector
	data     int
	label    string
}

func TestExplorePlanMove(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	ms, err := createNewExploreMotionService(ctx, logger, fakeBase, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)
	defer ms.Close(ctx)

	msStruct := ms.(*explore)

	// Create kinematic base
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultMotionCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)

	// Create empty worldState
	worldState, err := referenceframe.NewWorldState(nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cases := []struct {
		description              string
		destination              spatialmath.Pose
		expectedMotionPlanLength int
	}{
		{
			description:              "destination directly in front of base",
			destination:              spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			expectedMotionPlanLength: 2,
		},
		{
			description:              "destination directly behind the base",
			destination:              spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: -1000, Z: 0}),
			expectedMotionPlanLength: 2,
		},
		{
			description:              "destination off axis of base",
			destination:              spatialmath.NewPoseFromPoint(r3.Vector{X: 1000, Y: 1000, Z: 0}),
			expectedMotionPlanLength: 2,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			dest := referenceframe.NewPoseInFrame(testBaseName.Name, tt.destination)

			planInputs, err := msStruct.createMotionPlan(
				ctx,
				kb,
				dest,
				worldState,
				nil,
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(planInputs), test.ShouldEqual, tt.expectedMotionPlanLength)
		})
	}
}

func TestExploreCheckForObstacles(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Create fake camera
	fakeCamera, err := createFakeCamera(ctx, logger, testCameraName1.Name)
	test.That(t, err, test.ShouldBeNil)

	// Create fake base
	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create explore motion service
	ms, err := createNewExploreMotionService(ctx, logger, fakeBase, []camera.Camera{fakeCamera})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)
	defer ms.Close(ctx)

	msStruct := ms.(*explore)

	// Create kinematic base
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultMotionCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)

	// Create empty worldState
	worldState, err := referenceframe.NewWorldState(nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cases := []struct {
		description  string
		destination  spatialmath.Pose
		obstacle     []obstacleMetadata
		detection    bool
		detectionErr error
	}{
		{
			description: "no obstacles in view",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			detection:   false,
		},
		{
			description: "obstacle in path nearby",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: []obstacleMetadata{
				{
					position: r3.Vector{X: 0, Y: 300, Z: 0},
					data:     100,
					label:    "close_obstacle_in_path",
				},
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle in path farther away",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: []obstacleMetadata{
				{
					position: r3.Vector{X: 0, Y: 800, Z: 0},
					data:     100,
					label:    "far_obstacle_in_path",
				},
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle in path too far",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 10000, Z: 0}),
			obstacle: []obstacleMetadata{
				{
					position: r3.Vector{X: 0, Y: 1500, Z: 0},
					data:     100,
					label:    "far_obstacle_not_in_path",
				},
			},
			detection:    false,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle off axis in path",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 1000, Y: 1000, Z: 0}),
			obstacle: []obstacleMetadata{
				{
					position: r3.Vector{X: 500, Y: 500, Z: 0},
					data:     100,
					label:    "obstacle on diagonal",
				},
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle off axis not in path",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: []obstacleMetadata{
				{
					position: r3.Vector{X: 500, Y: 500, Z: 0},
					data:     100,
					label:    "close_obstacle_not_in_path",
				},
			},
			detection: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			// Create motionplan plan
			dest := referenceframe.NewPoseInFrame(testBaseName.Name, tt.destination)
			plan, err := msStruct.createMotionPlan(
				ctx,
				kb,
				dest,
				worldState,
				nil,
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
			test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

			// Create a vision service using provided obstacles and place it in an obstacle DetectorPair object
			visionService := createMockVisionService("", tt.obstacle)

			obstacleDetectors := []obstacleDetectorObject{
				{visionService: fakeCamera},
			}

			// Update and check worldState
			worldState, err := msStruct.generateTransientWorldState(ctx, obstacleDetectors)
			test.That(t, err, test.ShouldBeNil)

			if len(tt.obstacle) == 0 {
				fs, err := msStruct.fsService.FrameSystem(ctx, nil)
				test.That(t, err, test.ShouldBeNil)

				obstacles, err := worldState.ObstaclesInWorldFrame(fs, nil)
				test.That(t, err, test.ShouldBeNil)

				for _, obs := range tt.obstacle {
					collisionDetected, err := geometriesContainsPoint(obstacles.Geometries(), obs.position)
					test.That(t, err, test.ShouldBeNil)
					test.That(t, collisionDetected, test.ShouldBeTrue)
				}
			}

			// Run check obstacles in of plan path
			ctxTimeout, cancelFunc := context.WithTimeout(ctx, 1*time.Second)
			defer cancelFunc()

			msStruct.backgroundWorkers.Add(1)
			goutils.ManagedGo(func() {
				msStruct.checkForObstacles(ctxTimeout, obstacleDetectors, kb, plan, defaultMotionCfg.ObstaclePollingFreqHz)
			}, msStruct.backgroundWorkers.Done)

			resp := <-msStruct.obstacleResponseChan
			test.That(t, resp.success, test.ShouldEqual, tt.detection)
			if tt.detectionErr == nil {
				test.That(t, resp.err, test.ShouldBeNil)
			} else {
				test.That(t, resp.err, test.ShouldNotBeNil)
				test.That(t, resp.err.Error(), test.ShouldContainSubstring, tt.detectionErr.Error())
			}
		})
	}
}

func TestMultipleObstacleDetectors(t *testing.T) {
	ctx := context.Background()
	logger := golog.NewTestLogger(t)

	// Create fake cameras
	fakeCamera1, err := createFakeCamera(ctx, logger, testCameraName1.Name)
	test.That(t, err, test.ShouldBeNil)

	fakeCamera2, err := createFakeCamera(ctx, logger, testCameraName2.Name)
	test.That(t, err, test.ShouldBeNil)

	// Create fake base
	fakeBase, err := createFakeBase(ctx, logger)
	test.That(t, err, test.ShouldBeNil)

	// Create explore motion service
	ms, err := createNewExploreMotionService(ctx, logger, fakeBase, []camera.Camera{fakeCamera1, fakeCamera2})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)
	defer ms.Close(ctx)

	msStruct := ms.(*explore)

	// Create kinematic base
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultMotionCfg)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)

	// Create empty worldState
	worldState, err := referenceframe.NewWorldState(nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cases := []struct {
		description          string
		destination          spatialmath.Pose
		obstacleCamera1      obstacleMetadata
		obstacleCamera2      obstacleMetadata
		visionServiceCamLink []camera.Camera
	}{
		{
			description: "two independent vision services both detecting obstacles",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacleCamera1: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 300, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			obstacleCamera2: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 500, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			visionServiceCamLink: []camera.Camera{fakeCamera1, fakeCamera2},
		},
		{
			description: "two independent vision services only first detecting obstacle",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacleCamera1: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 300, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			obstacleCamera2:      obstacleMetadata{},
			visionServiceCamLink: []camera.Camera{fakeCamera1, fakeCamera2},
		},
		{
			description:     "two independent vision services only second detecting obstacle",
			destination:     spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacleCamera1: obstacleMetadata{},
			obstacleCamera2: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 300, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			visionServiceCamLink: []camera.Camera{fakeCamera1, fakeCamera2},
		},
		{
			description: "two vision services depending on same camera",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacleCamera1: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 300, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			obstacleCamera2:      obstacleMetadata{},
			visionServiceCamLink: []camera.Camera{fakeCamera1, fakeCamera2},
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			// Create motionplan plan
			dest := referenceframe.NewPoseInFrame(testBaseName.Name, tt.destination)
			plan, err := msStruct.createMotionPlan(
				ctx,
				kb,
				dest,
				worldState,
				nil,
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
			test.That(t, len(plan), test.ShouldBeGreaterThan, 0)

			// Create a vision service using provided obstacles and place it in an obstacle DetectorPair object
			var obstacleDetectors []obstacleDetectorObject
			for i, cam := range tt.visionServiceCamLink {
				var obstacles []obstacleMetadata
				if cam.Name().Name == testCameraName1.Name {
					obstacles = append(obstacles, tt.obstacleCamera1)
				}
				if cam.Name().Name == testCameraName2.Name {
					obstacles = append(obstacles, tt.obstacleCamera2)
				}
				visionService := createMockVisionService(fmt.Sprint(i), obstacles)
				obstacleDetectors = append(obstacleDetectors, obstacleDetectorObject{visionService: cam})
			}

			// Run check obstacles in of plan path
			ctxTimeout, cancelFunc := context.WithTimeout(ctx, 1*time.Second)
			defer cancelFunc()

			msStruct.backgroundWorkers.Add(1)
			goutils.ManagedGo(func() {
				msStruct.checkForObstacles(ctxTimeout, obstacleDetectors, kb, plan, defaultMotionCfg.ObstaclePollingFreqHz)
			}, msStruct.backgroundWorkers.Done)

			resp := <-msStruct.obstacleResponseChan
			test.That(t, resp.success, test.ShouldBeTrue)
			test.That(t, resp.err, test.ShouldNotBeNil)
			test.That(t, resp.err.Error(), test.ShouldContainSubstring, "found collision between positions")
		})
	}
}
