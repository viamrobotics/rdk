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

const (
	defaultLinearVelocityMMPerSec    = 200.
	defaultAngularVelocityDegsPerSec = 60.
)

type obstacleMetadata struct {
	position r3.Vector
	data     int
	label    string
}

func TestExploreCreateKBOpts(t *testing.T) {
	cases := []struct {
		description string
		extra       map[string]interface{}
		expectedErr error
	}{
		{
			description: "valid extra with angular_degs_per_sec and linear_m_per_sec",
			extra: map[string]interface{}{
				"angular_degs_per_sec": .1,
				"linear_m_per_sec":     .3,
			},
			expectedErr: nil,
		},
		{
			description: "invalid extra with only angular_degs_per_sec",
			extra: map[string]interface{}{
				"angular_degs_per_sec": .1,
			},
			expectedErr: nil,
		},
		{
			description: "invalid extra with only linear_m_per_sec",
			extra: map[string]interface{}{
				"linear_m_per_sec": .3,
			},
			expectedErr: nil,
		},
		{
			description: "invalid extra with invalid angular_degs_per_sec",
			extra: map[string]interface{}{
				"angular_degs_per_sec": "not_a_float",
			},
			expectedErr: errors.New("could not interpret angular_degs_per_sec field as float64"),
		},
		{
			description: "invalid extra with invalid linear_m_per_sec",
			extra: map[string]interface{}{
				"linear_m_per_sec": "not_a_float",
			},
			expectedErr: errors.New("could not interpret linear_m_per_sec field as float64"),
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			kbOpts, err := createKBOps(tt.extra)

			if tt.expectedErr != nil {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.expectedErr.Error())
				return
			}

			test.That(t, err, test.ShouldBeNil)
			test.That(t, kbOpts.NoSkidSteer, test.ShouldBeTrue)
			test.That(t, kbOpts.PositionOnlyMode, test.ShouldBeTrue)
			test.That(t, kbOpts.UsePTGs, test.ShouldBeFalse)

			if angularDegsPerSec, ok := tt.extra["angular_degs_per_sec"]; ok {
				test.That(t, kbOpts.AngularVelocityDegsPerSec, test.ShouldEqual, angularDegsPerSec)
			} else {
				test.That(t, kbOpts.AngularVelocityDegsPerSec, test.ShouldEqual, defaultAngularVelocityDegsPerSec)
			}

			if linearMPerSec, ok := tt.extra["linear_m_per_sec"]; ok {
				test.That(t, kbOpts.LinearVelocityMMPerSec, test.ShouldEqual, linearMPerSec)
			} else {
				test.That(t, kbOpts.LinearVelocityMMPerSec, test.ShouldEqual, defaultLinearVelocityMMPerSec)
			}
		})
	}
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
	kb, err := msStruct.createKinematicBase(ctx, fakeBase.Name(), defaultKBOptsExtra)
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
		{
			description:              "destination at origin",
			destination:              spatialmath.NewPoseFromPoint(r3.Vector{X: 0, Y: 0, Z: 0}),
			expectedMotionPlanLength: 1,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			planInputs, err := msStruct.createMotionPlan(
				ctx,
				kb,
				tt.destination,
				worldState,
				true,
				defaultKBOptsExtra,
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
			planInputs, err := msStruct.createMotionPlan(
				ctx,
				kb,
				tt.destination,
				worldState,
				true,
				defaultKBOptsExtra,
			)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, kb.Name().Name, test.ShouldEqual, testBaseName.Name)
			test.That(t, len(planInputs), test.ShouldBeGreaterThan, 0)

			var plan motionplan.Plan
			for _, inputs := range planInputs {
				input := make(map[string][]referenceframe.Input)
				input[kb.Name().Name] = inputs
				plan = append(plan, input)
			}

			// Create a vision service using provided obstacles and place it in an obstacle DetectorPair object
			visionService := createMockVisionService(tt.obstacle)

			obstacleDetectors := []obstacleDetectorPair{
				{visionService: {fakeCamera}},
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
				msStruct.checkForObstacles(ctxTimeout, obstacleDetectors, kb, plan, nil)
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
