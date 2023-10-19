package explore

import (
	"context"
	"testing"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	"go.viam.com/test"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

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
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultKBOptsExtra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)

	msStruct.kb = &kb

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
		{
			description:              "destination at origin",
			destination:              spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			expectedMotionPlanLength: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			planInputs, err := msStruct.createMotionPlan(ctx, tt.destination, worldState, true, defaultKBOptsExtra)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(planInputs), test.ShouldEqual, tt.expectedMotionPlanLength)
		})
	}
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
	ms, err := createNewExploreMotionService(ctx, logger, fakeBase, fakeCamera)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ms, test.ShouldNotBeNil)
	defer ms.Close(ctx)

	msStruct := ms.(*explore)

	// Create kinematic base
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultKBOptsExtra)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)

	msStruct.kb = &kb

	// Create empty worldState
	worldState, err := referenceframe.NewWorldState(nil, nil)
	test.That(t, err, test.ShouldBeNil)

	cases := []struct {
		description  string
		destination  spatialmath.Pose
		obstacle     obstacleMetadata
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
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 300, Z: 0},
				data:     100,
				label:    "close_obstacle_in_path",
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle in path farther away",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 800, Z: 0},
				data:     100,
				label:    "far_obstacle_in_path",
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle in path too far",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 10000, Z: 0}),
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 0, Y: 1500, Z: 0},
				data:     100,
				label:    "far_obstacle_not_in_path",
			},
			detection:    false,
			detectionErr: errors.New("found collision between positions"),
		},
		{
			description: "obstacle not in path",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 1000, Z: 0}),
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 500, Y: 500, Z: 0},
				data:     100,
				label:    "close_obstacle_not_in_path",
			},
			detection: false,
		},
		{
			description: "obstacle on diagonal",
			destination: spatialmath.NewPoseFromPoint(r3.Vector{X: 1000, Y: 1000, Z: 0}),
			obstacle: obstacleMetadata{
				position: r3.Vector{X: 500, Y: 500, Z: 0},
				data:     100,
				label:    "obstacle on diagonal",
			},
			detection:    true,
			detectionErr: errors.New("found collision between positions"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			// Create motionplan plan
			planInputs, err := msStruct.createMotionPlan(ctx, tt.destination, worldState, true, defaultKBOptsExtra)
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
			msStruct.visionService = createMockVisionService(tt.obstacle)

			worldState, err := msStruct.updateWorldState(ctx)
			test.That(t, err, test.ShouldBeNil)

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

			// Run check obstacles in of plan path
			ctxTimeout, cancelFunc := context.WithTimeout(ctx, 1*time.Second)
			defer cancelFunc()

			msStruct.backgroundWorkers.Add(1)
			goutils.ManagedGo(func() {
				msStruct.checkForObstacles(ctxTimeout, plan)
			}, msStruct.backgroundWorkers.Done)

			resp := <-msStruct.obstacleChan
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
