package builtin

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	"github.com/google/uuid"
	geo "github.com/kellydunn/golang-geo"
	"go.uber.org/atomic"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	baseFake "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/camera"
	_ "go.viam.com/rdk/components/camera/fake"
	"go.viam.com/rdk/components/movementsensor"
	_ "go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	robotimpl "go.viam.com/rdk/robot/impl"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/vision"
	_ "go.viam.com/rdk/services/vision/colordetector"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

type startWaypointState struct {
	ns             navigation.Service
	injectMS       *inject.MotionService
	base           base.Base
	movementSensor *inject.MovementSensor
	closeFunc      func()
	sync.RWMutex
	pws   []motion.PlanWithStatus
	mogrs []motion.MoveOnGlobeReq
	sprs  []motion.StopPlanReq
}

func setupNavigationServiceFromConfig(t *testing.T, configFilename string) (navigation.Service, func()) {
	t.Helper()
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg, err := config.Read(ctx, configFilename, logger)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cfg.Ensure(false, logger), test.ShouldBeNil)
	myRobot, err := robotimpl.New(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	svc, err := navigation.FromRobot(myRobot, "test_navigation")
	test.That(t, err, test.ShouldBeNil)
	return svc, func() {
		myRobot.Close(context.Background())
	}
}

func TestValidateConfig(t *testing.T) {
	path := ""

	cases := []struct {
		description string
		cfg         Config
		numDeps     int
		expectedErr error
	}{
		{
			description: "valid config default map type (GPS) give",
			cfg: Config{
				BaseName:           "base",
				MovementSensorName: "localizer",
			},
			numDeps:     3,
			expectedErr: nil,
		},
		{
			description: "valid config for map_type none given",
			cfg: Config{
				BaseName: "base",
				MapType:  "None",
			},
			numDeps:     2,
			expectedErr: nil,
		},
		{
			description: "valid config for map_type GPS given",
			cfg: Config{
				BaseName:           "base",
				MapType:            "GPS",
				MovementSensorName: "localizer",
			},
			numDeps:     3,
			expectedErr: nil,
		},
		{
			description: "invalid config no base",
			cfg:         Config{},
			numDeps:     0,
			expectedErr: resource.NewConfigValidationFieldRequiredError(path, "base"),
		},
		{
			description: "invalid config no movement_sensor given for map type GPS",
			cfg: Config{
				BaseName: "base",
				MapType:  "GPS",
			},
			numDeps:     0,
			expectedErr: resource.NewConfigValidationFieldRequiredError(path, "movement_sensor"),
		},
		{
			description: "invalid config negative degs_per_sec",
			cfg: Config{
				BaseName:           "base",
				MovementSensorName: "localizer",
				DegPerSec:          -1,
			},
			numDeps:     0,
			expectedErr: errNegativeDegPerSec,
		},
		{
			description: "invalid config negative meters_per_sec",
			cfg: Config{
				BaseName:           "base",
				MovementSensorName: "localizer",
				MetersPerSec:       -1,
			},
			numDeps:     0,
			expectedErr: errNegativeMetersPerSec,
		},
		{
			description: "invalid config negative position_polling_frequency_hz",
			cfg: Config{
				BaseName:                   "base",
				MovementSensorName:         "localizer",
				PositionPollingFrequencyHz: -1,
			},
			numDeps:     0,
			expectedErr: errNegativePositionPollingFrequencyHz,
		},
		{
			description: "invalid config negative obstacle_polling_frequency_hz",
			cfg: Config{
				BaseName:                   "base",
				MovementSensorName:         "localizer",
				ObstaclePollingFrequencyHz: -1,
			},
			numDeps:     0,
			expectedErr: errNegativeObstaclePollingFrequencyHz,
		},
		{
			description: "invalid config negative plan_deviation_m",
			cfg: Config{
				BaseName:           "base",
				MovementSensorName: "localizer",
				PlanDeviationM:     -1,
			},
			numDeps:     0,
			expectedErr: errNegativePlanDeviationM,
		},
		{
			description: "invalid config negative replan_cost_factor",
			cfg: Config{
				BaseName:           "base",
				MovementSensorName: "localizer",
				ReplanCostFactor:   -1,
			},
			numDeps:     0,
			expectedErr: errNegativeReplanCostFactor,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			deps, err := tt.cfg.Validate(path)
			if tt.expectedErr == nil {
				test.That(t, err, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldContainSubstring, tt.expectedErr.Error())
			}
			test.That(t, len(deps), test.ShouldEqual, tt.numDeps)
		})
	}
}

func TestNew(t *testing.T) {
	ctx := context.Background()

	svc, closeNavSvc := setupNavigationServiceFromConfig(t, "../data/nav_no_map_cfg_minimal.json")

	t.Run("checking defaults have been set", func(t *testing.T) {
		svcStruct := svc.(*builtIn)

		test.That(t, svcStruct.base.Name().Name, test.ShouldEqual, "test_base")
		test.That(t, svcStruct.motionService.Name().Name, test.ShouldEqual, "builtin")

		test.That(t, svcStruct.mapType, test.ShouldEqual, navigation.NoMap)
		test.That(t, svcStruct.mode, test.ShouldEqual, navigation.ModeManual)
		test.That(t, svcStruct.replanCostFactor, test.ShouldEqual, defaultReplanCostFactor)

		test.That(t, svcStruct.storeType, test.ShouldEqual, string(navigation.StoreTypeMemory))
		test.That(t, svcStruct.store, test.ShouldResemble, navigation.NewMemoryNavigationStore())

		test.That(t, svcStruct.motionCfg.ObstacleDetectors, test.ShouldBeNil)
		test.That(t, svcStruct.motionCfg.AngularDegsPerSec, test.ShouldEqual, defaultAngularVelocityDegsPerSec)
		test.That(t, svcStruct.motionCfg.LinearMPerSec, test.ShouldEqual, defaultLinearVelocityMPerSec)
		test.That(t, svcStruct.motionCfg.PositionPollingFreqHz, test.ShouldEqual, defaultPositionPollingFrequencyHz)
		test.That(t, svcStruct.motionCfg.ObstaclePollingFreqHz, test.ShouldEqual, defaultObstaclePollingFrequencyHz)
		test.That(t, svcStruct.motionCfg.PlanDeviationMM, test.ShouldEqual, defaultPlanDeviationM*1e3)
	})

	t.Run("setting parameters for None map_type", func(t *testing.T) {
		cfg := &Config{
			BaseName:           "base",
			MapType:            "None",
			MovementSensorName: "movement_sensor",
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):                      inject.NewBase("new_base"),
			resource.NewName(motion.API, "builtin"):                 inject.NewMotionService("new_motion"),
			resource.NewName(movementsensor.API, "movement_sensor"): inject.NewMovementSensor("movement_sensor"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeNil)
		svcStruct := svc.(*builtIn)

		test.That(t, svcStruct.mapType, test.ShouldEqual, navigation.NoMap)
		test.That(t, svcStruct.base.Name().Name, test.ShouldEqual, "new_base")
		test.That(t, svcStruct.motionService.Name().Name, test.ShouldEqual, "new_motion")
		test.That(t, svcStruct.movementSensor, test.ShouldBeNil)
	})

	t.Run("setting parameters for GPS map_type", func(t *testing.T) {
		cfg := &Config{
			BaseName:           "base",
			MapType:            "GPS",
			MovementSensorName: "movement_sensor",
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):                      inject.NewBase("new_base"),
			resource.NewName(motion.API, "builtin"):                 inject.NewMotionService("new_motion"),
			resource.NewName(movementsensor.API, "movement_sensor"): inject.NewMovementSensor("movement_sensor"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeNil)
		svcStruct := svc.(*builtIn)

		test.That(t, svcStruct.mapType, test.ShouldEqual, navigation.GPSMap)
		test.That(t, svcStruct.base.Name().Name, test.ShouldEqual, "new_base")
		test.That(t, svcStruct.motionService.Name().Name, test.ShouldEqual, "new_motion")
		test.That(t, svcStruct.movementSensor.Name().Name, test.ShouldEqual, cfg.MovementSensorName)
	})

	t.Run("setting motion parameters", func(t *testing.T) {
		cfg := &Config{
			BaseName:                   "base",
			MapType:                    "None",
			DegPerSec:                  1,
			MetersPerSec:               2,
			PositionPollingFrequencyHz: 3,
			ObstaclePollingFrequencyHz: 4,
			PlanDeviationM:             5,
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					VisionServiceName: "vision",
					CameraName:        "camera",
				},
			},
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):      &inject.Base{},
			resource.NewName(camera.API, "camera"):  inject.NewCamera("camera"),
			resource.NewName(motion.API, "builtin"): inject.NewMotionService("motion"),
			resource.NewName(vision.API, "vision"):  inject.NewVisionService("vision"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeNil)
		svcStruct := svc.(*builtIn)

		test.That(t, len(svcStruct.motionCfg.ObstacleDetectors), test.ShouldEqual, 1)
		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].VisionServiceName.Name, test.ShouldEqual, "vision")
		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].CameraName.Name, test.ShouldEqual, "camera")

		test.That(t, svcStruct.motionCfg.AngularDegsPerSec, test.ShouldEqual, cfg.DegPerSec)
		test.That(t, svcStruct.motionCfg.LinearMPerSec, test.ShouldEqual, cfg.MetersPerSec)
		test.That(t, svcStruct.motionCfg.PositionPollingFreqHz, test.ShouldEqual, cfg.PositionPollingFrequencyHz)
		test.That(t, svcStruct.motionCfg.ObstaclePollingFreqHz, test.ShouldEqual, cfg.ObstaclePollingFrequencyHz)
		test.That(t, svcStruct.motionCfg.PlanDeviationMM, test.ShouldEqual, cfg.PlanDeviationM*1e3)
	})

	t.Run("setting additional parameters", func(t *testing.T) {
		cfg := &Config{
			BaseName:         "base",
			MapType:          "None",
			ReplanCostFactor: 1,
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					VisionServiceName: "vision",
					CameraName:        "camera",
				},
			},
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):      &inject.Base{},
			resource.NewName(camera.API, "camera"):  inject.NewCamera("camera"),
			resource.NewName(motion.API, "builtin"): inject.NewMotionService("motion"),
			resource.NewName(vision.API, "vision"):  inject.NewVisionService("vision"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeNil)
		svcStruct := svc.(*builtIn)

		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].VisionServiceName.Name, test.ShouldEqual, "vision")
		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].CameraName.Name, test.ShouldEqual, "camera")
		test.That(t, svcStruct.replanCostFactor, test.ShouldEqual, cfg.ReplanCostFactor)
	})

	t.Run("base missing from deps", func(t *testing.T) {
		expectedErr := resource.DependencyNotFoundError(base.Named(""))
		cfg := &Config{}
		deps := resource.Dependencies{}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	t.Run("motion missing from deps", func(t *testing.T) {
		expectedErr := resource.DependencyNotFoundError(motion.Named("builtin"))
		cfg := &Config{
			BaseName: "base",
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"): &inject.Base{},
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	t.Run("movement sensor missing from deps", func(t *testing.T) {
		expectedErr := resource.DependencyNotFoundError(movementsensor.Named(""))
		cfg := &Config{
			BaseName: "base",
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):      &inject.Base{},
			resource.NewName(motion.API, "builtin"): inject.NewMotionService("motion"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	t.Run("vision missing from deps", func(t *testing.T) {
		expectedErr := resource.DependencyNotFoundError(vision.Named(""))
		cfg := &Config{
			BaseName:           "base",
			MovementSensorName: "movement_sensor",
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					CameraName: "camera",
				},
			},
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):                      &inject.Base{},
			resource.NewName(motion.API, "builtin"):                 inject.NewMotionService("motion"),
			resource.NewName(camera.API, "camera"):                  inject.NewCamera("camera"),
			resource.NewName(movementsensor.API, "movement_sensor"): inject.NewMovementSensor("movement_sensor"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	t.Run("camera missing from deps", func(t *testing.T) {
		expectedErr := resource.DependencyNotFoundError(camera.Named(""))
		cfg := &Config{
			BaseName:           "base",
			MovementSensorName: "movement_sensor",
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					VisionServiceName: "vision",
				},
			},
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):                      &inject.Base{},
			resource.NewName(motion.API, "builtin"):                 inject.NewMotionService("motion"),
			resource.NewName(vision.API, "vision"):                  inject.NewVisionService("vision"),
			resource.NewName(movementsensor.API, "movement_sensor"): inject.NewMovementSensor("movement_sensor"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeError, expectedErr)
	})

	t.Run("necessary for MoveOnGlobe", func(t *testing.T) {
		cfg := &Config{
			BaseName:           "base",
			MovementSensorName: "movement_sensor",
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					VisionServiceName: "vision",
					CameraName:        "camera",
				},
			},
		}
		deps := resource.Dependencies{
			resource.NewName(base.API, "base"):                      &inject.Base{},
			resource.NewName(motion.API, "builtin"):                 inject.NewMotionService("motion"),
			resource.NewName(vision.API, "vision"):                  inject.NewVisionService("vision"),
			resource.NewName(camera.API, "camera"):                  inject.NewCamera("camera"),
			resource.NewName(movementsensor.API, "movement_sensor"): inject.NewMovementSensor("movement_sensor"),
		}

		err := svc.Reconfigure(ctx, deps, resource.Config{ConvertedAttributes: cfg})
		test.That(t, err, test.ShouldBeNil)
		svcStruct := svc.(*builtIn)

		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].VisionServiceName.Name, test.ShouldEqual, "vision")
		test.That(t, svcStruct.motionCfg.ObstacleDetectors[0].CameraName.Name, test.ShouldEqual, "camera")
	})

	closeNavSvc()
}

func TestSetMode(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		description string
		cfg         string
		mapType     navigation.MapType
		mode        navigation.Mode
		expectedErr error
	}{
		{
			description: "setting mode to manual when map_type is None",
			cfg:         "../data/nav_no_map_cfg.json",
			mapType:     navigation.NoMap,
			mode:        navigation.ModeManual,
			expectedErr: nil,
		},
		{
			description: "setting mode to waypoint when map_type is None",
			cfg:         "../data/nav_no_map_cfg.json",
			mapType:     navigation.NoMap,
			mode:        navigation.ModeWaypoint,
			expectedErr: errors.New("Waypoint mode is unavailable for map type None"),
		},
		{
			description: "setting mode to explore when map_type is None",
			cfg:         "../data/nav_no_map_cfg.json",
			mapType:     navigation.NoMap,
			mode:        navigation.ModeExplore,
			expectedErr: nil,
		},
		{
			description: "setting mode to explore when map_type is None and no vision service is configured",
			cfg:         "../data/nav_no_map_cfg_minimal.json",
			mapType:     navigation.GPSMap,
			mode:        navigation.ModeExplore,
			expectedErr: errors.New("explore mode requires at least one vision service"),
		},
		{
			description: "setting mode to manual when map_type is GPS",
			cfg:         "../data/nav_cfg.json",
			mapType:     navigation.GPSMap,
			mode:        navigation.ModeManual,
			expectedErr: nil,
		},
		{
			description: "setting mode to waypoint when map_type is GPS",
			cfg:         "../data/nav_cfg.json",
			mapType:     navigation.GPSMap,
			mode:        navigation.ModeWaypoint,
			expectedErr: nil,
		},
		{
			description: "setting mode to explore when map_type is GPS",
			cfg:         "../data/nav_cfg.json",
			mapType:     navigation.GPSMap,
			mode:        navigation.ModeExplore,
			expectedErr: nil,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			ns, teardown := setupNavigationServiceFromConfig(t, tt.cfg)
			defer teardown()

			navMode, err := ns.Mode(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

			err = ns.SetMode(ctx, tt.mode, nil)
			if tt.expectedErr == nil {
				test.That(t, err, test.ShouldEqual, tt.expectedErr)
			} else {
				test.That(t, err, test.ShouldNotBeNil)
				test.That(t, err.Error(), test.ShouldEqual, tt.expectedErr.Error())
			}
		})
	}
}

func TestNavSetup(t *testing.T) {
	ns, teardown := setupNavigationServiceFromConfig(t, "../data/nav_cfg.json")
	defer teardown()
	ctx := context.Background()

	navMode, err := ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeManual)

	err = ns.SetMode(ctx, navigation.ModeWaypoint, nil)
	test.That(t, err, test.ShouldBeNil)
	navMode, err = ns.Mode(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, navMode, test.ShouldEqual, navigation.ModeWaypoint)

	// Prevent race
	err = ns.SetMode(ctx, navigation.ModeManual, nil)
	test.That(t, err, test.ShouldBeNil)

	geoPose, err := ns.Location(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	expectedGeoPose := spatialmath.NewGeoPose(geo.NewPoint(40.7, -73.98), 25.)
	test.That(t, geoPose, test.ShouldResemble, expectedGeoPose)

	wayPt, err := ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, wayPt, test.ShouldBeEmpty)

	pt := geo.NewPoint(0, 0)
	err = ns.AddWaypoint(ctx, pt, nil)
	test.That(t, err, test.ShouldBeNil)

	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 1)

	id := wayPt[0].ID
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)
	wayPt, err = ns.Waypoints(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(wayPt), test.ShouldEqual, 0)

	// Calling RemoveWaypoint on an already removed waypoint doesn't return an error
	err = ns.RemoveWaypoint(ctx, id, nil)
	test.That(t, err, test.ShouldBeNil)

	obs, err := ns.Obstacles(ctx, nil)
	test.That(t, len(obs), test.ShouldEqual, 1)
	test.That(t, err, test.ShouldBeNil)

	test.That(t, len(ns.(*builtIn).motionCfg.ObstacleDetectors), test.ShouldEqual, 1)

	_, err = ns.Paths(ctx, nil)
	test.That(t, err, test.ShouldBeError, resource.NewNotFoundError(base.Named("test_base")))
}

func setupStartWaypoint(ctx context.Context, t *testing.T, logger logging.Logger) startWaypointState {
	fakeBase, err := baseFake.NewBase(ctx, nil, resource.Config{
		Name:  "test_base",
		API:   base.API,
		Frame: &referenceframe.LinkConfig{Geometry: &spatialmath.GeometryConfig{R: 100}},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	injectMovementSensor := inject.NewMovementSensor("test_movement")
	visionService := inject.NewVisionService("vision")
	camera := inject.NewCamera("camera")
	config := resource.Config{
		ConvertedAttributes: &Config{
			Store: navigation.StoreConfig{
				Type: navigation.StoreTypeMemory,
			},
			BaseName:           "test_base",
			MovementSensorName: "test_movement",
			MotionServiceName:  "test_motion",
			DegPerSec:          1,
			MetersPerSec:       1,
			ObstacleDetectors: []*ObstacleDetectorNameConfig{
				{
					VisionServiceName: "vision",
					CameraName:        "camera",
				},
			},
		},
	}
	injectMS := inject.NewMotionService("test_motion")
	deps := resource.Dependencies{
		injectMS.Name():             injectMS,
		fakeBase.Name():             fakeBase,
		injectMovementSensor.Name(): injectMovementSensor,
		visionService.Name():        visionService,
		camera.Name():               camera,
	}
	ns, err := NewBuiltIn(ctx, deps, config, logger)
	test.That(t, err, test.ShouldBeNil)
	return startWaypointState{
		ns:             ns,
		injectMS:       injectMS,
		base:           fakeBase,
		movementSensor: injectMovementSensor,
		closeFunc:      func() { test.That(t, ns.Close(context.Background()), test.ShouldBeNil) },
	}
}

func TestStartWaypoint(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	t.Run("Reach waypoints successfully", func(t *testing.T) {
		s := setupStartWaypoint(ctx, t, logger)
		defer s.closeFunc()

		// we expect 2 executions to be generated
		executionIDs := []uuid.UUID{
			uuid.New(),
			uuid.New(),
		}
		counter := atomic.NewInt32(-1)
		var wg sync.WaitGroup
		defer wg.Wait()
		// MoveOnGlobeNew will behave as if it created a new plan & queue up a goroutine which will then behave as if the plan succeeded
		s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
			executionID := executionIDs[(counter.Inc())]
			s.Lock()
			defer s.Unlock()
			if s.mogrs == nil {
				s.mogrs = []motion.MoveOnGlobeReq{}
			}
			s.mogrs = append(s.mogrs, req)
			s.pws = []motion.PlanWithStatus{
				{
					Plan: motion.Plan{
						ExecutionID: executionID,
					},
					StatusHistory: []motion.PlanStatus{
						{State: motion.PlanStateInProgress},
					},
				},
			}
			wg.Add(1)
			utils.ManagedGo(func() {
				s.Lock()
				defer s.Unlock()
				for i, p := range s.pws {
					if p.Plan.ExecutionID == executionID {
						succeeded := []motion.PlanStatus{{State: motion.PlanStateSucceeded}}
						p.StatusHistory = append(succeeded, p.StatusHistory...)
						s.pws[i] = p
						return
					}
				}
				t.Error("MoveOnGlobeNew called unexpectedly")
			}, wg.Done)
			return executionID.String(), nil
		}

		s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			s.RLock()
			defer s.RUnlock()
			history := make([]motion.PlanWithStatus, len(s.pws))
			copy(history, s.pws)
			return history, nil
		}

		s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
			s.Lock()
			defer s.Unlock()
			if s.sprs == nil {
				s.sprs = []motion.StopPlanReq{}
			}
			s.sprs = append(s.sprs, req)
			return nil
		}

		pt1 := geo.NewPoint(1, 0)
		err := s.ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		pt2 := geo.NewPoint(3, 1)
		err = s.ns.AddWaypoint(ctx, pt2, nil)
		test.That(t, err, test.ShouldBeNil)

		wps, err := s.ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 2)

		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		expectedMotionCfg := &motion.MotionConfiguration{
			PositionPollingFreqHz: 2,
			ObstaclePollingFreqHz: 2,
			PlanDeviationMM:       1e+12,
			LinearMPerSec:         1,
			AngularDegsPerSec:     1,
			ObstacleDetectors: []motion.ObstacleDetectorName{
				{
					VisionServiceName: vision.Named("vision"),
					CameraName:        camera.Named("camera"),
				},
			},
		}
		// poll till waypoints is of length 0
		// query PlanHistory & confirm that you get back UUID 2
		timeoutCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*500)
		defer cancelFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Error("test timed out")
				t.FailNow()
			}
			// once all waypoints have been consumed
			wps, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if len(wps) == 0 {
				s.RLock()
				// wait until StopPlan has been called twice
				if len(s.sprs) == 2 {
					// MoveOnGlobeNew was called twice, once for each waypoint
					test.That(t, len(s.mogrs), test.ShouldEqual, 2)
					test.That(t, s.mogrs[0].ComponentName, test.ShouldResemble, s.base.Name())
					test.That(t, math.IsNaN(s.mogrs[0].Heading), test.ShouldBeTrue)
					test.That(t, s.mogrs[0].MovementSensorName, test.ShouldResemble, s.movementSensor.Name())
					test.That(t, s.mogrs[0].Extra, test.ShouldResemble, map[string]interface{}{"motion_profile": "position_only"})
					test.That(t, s.mogrs[0].MotionCfg, test.ShouldResemble, expectedMotionCfg)
					test.That(t, s.mogrs[0].Obstacles, test.ShouldBeNil)
					// waypoint 1
					test.That(t, s.mogrs[0].Destination, test.ShouldResemble, pt1)

					test.That(t, s.mogrs[1].ComponentName, test.ShouldResemble, s.base.Name())
					test.That(t, math.IsNaN(s.mogrs[1].Heading), test.ShouldBeTrue)
					test.That(t, s.mogrs[1].MovementSensorName, test.ShouldResemble, s.movementSensor.Name())
					test.That(t, s.mogrs[1].Extra, test.ShouldResemble, map[string]interface{}{"motion_profile": "position_only"})
					test.That(t, s.mogrs[1].MotionCfg, test.ShouldResemble, expectedMotionCfg)
					test.That(t, s.mogrs[1].Obstacles, test.ShouldBeNil)
					// waypoint 2
					test.That(t, s.mogrs[1].Destination, test.ShouldResemble, pt2)

					// PlanStop called twice, once for each waypoint
					test.That(t, s.sprs[0].ComponentName, test.ShouldResemble, s.base.Name())
					test.That(t, s.sprs[1].ComponentName, test.ShouldResemble, s.base.Name())

					// Motion reports that the last execution succeeded
					ph, err := s.injectMS.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: s.base.Name()})
					test.That(t, err, test.ShouldBeNil)
					test.That(t, len(ph), test.ShouldEqual, 1)
					test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionIDs[1])
					test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 2)
					test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
					s.RUnlock()
					break
				}
				s.RUnlock()
			}
		}
	})

	t.Run("SetMode's extra field is passed to MoveOnGlobe, with the default motoin profile of position_only", func(t *testing.T) {
		s := setupStartWaypoint(ctx, t, logger)
		defer s.closeFunc()

		executionID := uuid.New()
		mogCalled := make(chan struct{}, 1)
		s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}

			s.Lock()
			if s.mogrs == nil {
				s.mogrs = []motion.MoveOnGlobeReq{}
			}
			s.mogrs = append(s.mogrs, req)
			s.Unlock()
			mogCalled <- struct{}{}
			return executionID.String(), nil
		}

		// PlanHistory always reports execution is in progress
		s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return []motion.PlanWithStatus{
				{
					Plan: motion.Plan{
						ExecutionID: executionID,
					},
					StatusHistory: []motion.PlanStatus{
						{State: motion.PlanStateInProgress},
					},
				},
			}, nil
		}

		s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			return nil
		}

		pt1 := geo.NewPoint(1, 0)
		err := s.ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		wps, err := s.ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, len(wps), test.ShouldEqual, 1)

		// SetMode with nil extra
		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)
		<-mogCalled

		err = s.ns.SetMode(ctx, navigation.ModeManual, nil)
		test.That(t, err, test.ShouldBeNil)

		// SetMode with empty map
		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{})
		test.That(t, err, test.ShouldBeNil)
		<-mogCalled

		err = s.ns.SetMode(ctx, navigation.ModeManual, nil)
		test.That(t, err, test.ShouldBeNil)

		// SetMode with motion_profile set
		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, map[string]interface{}{"motion_profile": "some_other_motion_profile"})
		test.That(t, err, test.ShouldBeNil)
		<-mogCalled

		// poll till s.mogrs has length 3
		timeoutCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*500)
		defer cancelFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Error("test timed out")
				t.FailNow()
			}
			s.RLock()
			if len(s.mogrs) == 3 {
				// MoveOnGlobeNew was called twice, once for each waypoint
				test.That(t, s.mogrs[0].Extra, test.ShouldResemble, map[string]interface{}{"motion_profile": "position_only"})
				test.That(t, s.mogrs[1].Extra, test.ShouldResemble, map[string]interface{}{"motion_profile": "position_only"})
				test.That(t, s.mogrs[2].Extra, test.ShouldResemble, map[string]interface{}{"motion_profile": "some_other_motion_profile"})
				s.RUnlock()
				break
			}
			s.RUnlock()
		}
	})

	t.Run("motion errors result in retrying the current waypoint", func(t *testing.T) {
		s := setupStartWaypoint(ctx, t, logger)
		defer s.closeFunc()

		mogCounter := atomic.NewInt32(0)
		planHistoryCounter := atomic.NewInt32(0)
		executionID := uuid.New()
		var wg sync.WaitGroup
		s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			s.Lock()
			defer s.Unlock()
			if s.mogrs == nil {
				s.mogrs = []motion.MoveOnGlobeReq{}
			}
			s.mogrs = append(s.mogrs, req)

			// first call returns motion erro
			// second call returns context cancelled
			// third call returns success
			switch mogCounter.Inc() {
			case 1:
				return "", errors.New("motion error")
			case 2:
				return "", context.Canceled
			default:
				s.pws = []motion.PlanWithStatus{
					{
						Plan: motion.Plan{
							ExecutionID: executionID,
						},
						StatusHistory: []motion.PlanStatus{
							{State: motion.PlanStateInProgress},
						},
					},
				}
				wg.Add(1)
				utils.ManagedGo(func() {
					s.Lock()
					defer s.Unlock()
					for i, p := range s.pws {
						if p.Plan.ExecutionID == executionID {
							succeeded := []motion.PlanStatus{{State: motion.PlanStateSucceeded}}
							p.StatusHistory = append(succeeded, p.StatusHistory...)
							s.pws[i] = p
							return
						}
					}
					t.Error("MoveOnGlobeNew called unexpectedly")
				}, wg.Done)
				return executionID.String(), nil
			}
		}

		s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			switch planHistoryCounter.Inc() {
			case 1:
				return nil, errors.New("motion error")
			case 2:
				return nil, context.Canceled
			default:
				s.RLock()
				defer s.RUnlock()
				history := make([]motion.PlanWithStatus, len(s.pws))
				copy(history, s.pws)
				return history, nil
			}
		}

		s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			s.Lock()
			defer s.Unlock()
			if s.sprs == nil {
				s.sprs = []motion.StopPlanReq{}
			}
			s.sprs = append(s.sprs, req)
			return nil
		}

		pt1 := geo.NewPoint(1, 0)
		err := s.ns.AddWaypoint(ctx, pt1, nil)
		test.That(t, err, test.ShouldBeNil)

		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		timeoutCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*500)
		defer cancelFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Error("test timed out")
				t.FailNow()
			}
			// once all waypoints have been consumed
			wps, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if len(wps) == 0 {
				s.RLock()
				// wait until StopPlan has been called twice
				if len(s.sprs) == 3 {
					test.That(t, len(s.mogrs), test.ShouldEqual, 5)
					test.That(t, s.mogrs[0].Destination, test.ShouldResemble, pt1)
					test.That(t, s.mogrs[1].Destination, test.ShouldResemble, pt1)
					test.That(t, s.mogrs[2].Destination, test.ShouldResemble, pt1)
					test.That(t, s.mogrs[3].Destination, test.ShouldResemble, pt1)
					test.That(t, s.mogrs[4].Destination, test.ShouldResemble, pt1)

					// // Motion reports that the last execution succeeded
					ph, err := s.injectMS.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: s.base.Name()})
					test.That(t, err, test.ShouldBeNil)
					test.That(t, len(ph), test.ShouldEqual, 1)
					test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionID)
					test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 2)
					test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
					s.RUnlock()
					break
				}
				s.RUnlock()
			}
		}
	})

	// Calling SetMode cancels current and future MoveOnGlobe calls
	cases := []struct {
		description string
		mode        navigation.Mode
	}{
		{
			description: "Calling SetMode manual cancels context of current and future motion calls",
			mode:        navigation.ModeManual,
		},
		{
			description: "Calling SetMode explore cancels context of current and future motion calls",
			mode:        navigation.ModeExplore,
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			s := setupStartWaypoint(ctx, t, logger)
			defer s.closeFunc()

			executionID := uuid.New()
			s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
				s.Lock()
				defer s.Unlock()
				if s.mogrs == nil {
					s.mogrs = []motion.MoveOnGlobeReq{}
				}
				s.mogrs = append(s.mogrs, req)
				s.pws = []motion.PlanWithStatus{
					{
						Plan: motion.Plan{
							ExecutionID: executionID,
						},
						StatusHistory: []motion.PlanStatus{
							{State: motion.PlanStateInProgress},
						},
					},
				}
				return executionID.String(), nil
			}

			counter := atomic.NewInt32(0)
			modeFlag := make(chan struct{}, 1)
			s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
				s.RLock()
				defer s.RUnlock()
				history := make([]motion.PlanWithStatus, len(s.pws))
				copy(history, s.pws)
				count := counter.Inc()
				if count == 2 {
					modeFlag <- struct{}{}
				}
				return history, nil
			}

			s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
				s.Lock()
				defer s.Unlock()
				if s.sprs == nil {
					s.sprs = []motion.StopPlanReq{}
				}
				s.sprs = append(s.sprs, req)
				for i, p := range s.pws {
					if p.Plan.ExecutionID == executionID {
						stopped := []motion.PlanStatus{{State: motion.PlanStateStopped}}
						p.StatusHistory = append(stopped, p.StatusHistory...)
						s.pws[i] = p
						return nil
					}
				}
				return nil
			}

			// Set manual mode to ensure waypoint loop from prior test exits
			points := []*geo.Point{geo.NewPoint(1, 2), geo.NewPoint(2, 3)}
			for _, pt := range points {
				err := s.ns.AddWaypoint(ctx, pt, nil)
				test.That(t, err, test.ShouldBeNil)
			}
			wpBefore, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(wpBefore), test.ShouldEqual, 2)
			test.That(t, wpBefore[0].ToPoint(), test.ShouldResemble, points[0])
			test.That(t, wpBefore[1].ToPoint(), test.ShouldResemble, points[1])

			// start navigation - set ModeWaypoint first to ensure navigation starts up
			err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
			test.That(t, err, test.ShouldBeNil)

			// wait until MoveOnGlobe has been called & are polling the history
			<-modeFlag

			// Change the mode --> cancels the context
			err = s.ns.SetMode(ctx, tt.mode, nil)
			test.That(t, err, test.ShouldBeNil)

			// observe that no waypoints are deleted
			wpAfter, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, wpAfter, test.ShouldResemble, wpBefore)

			// check the last state of the execution
			ph, err := s.injectMS.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: s.base.Name()})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, len(ph), test.ShouldEqual, 1)
			test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 2)
			// The history reports that the terminal state of the execution is stopped
			test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateStopped)
		})
	}

	t.Run("Calling RemoveWaypoint on the waypoint in progress cancels current MoveOnGlobe call", func(t *testing.T) {
		s := setupStartWaypoint(ctx, t, logger)
		defer s.closeFunc()

		// we expect 2 executions to be generated
		executionIDs := []uuid.UUID{
			uuid.New(),
			uuid.New(),
		}
		counter := atomic.NewInt32(-1)
		var wg sync.WaitGroup
		defer wg.Wait()
		// MoveOnGlobeNew will behave as if it created a new plan & queue up a goroutine which will then behave as if the plan succeeded
		s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
			executionID := executionIDs[(counter.Inc())]
			s.Lock()
			defer s.Unlock()
			if s.mogrs == nil {
				s.mogrs = []motion.MoveOnGlobeReq{}
			}
			s.mogrs = append(s.mogrs, req)
			s.pws = []motion.PlanWithStatus{
				{
					Plan: motion.Plan{
						ExecutionID: executionID,
					},
					StatusHistory: []motion.PlanStatus{
						{State: motion.PlanStateInProgress},
					},
				},
			}
			// only succeed for the second execution
			if executionID == executionIDs[1] {
				wg.Add(1)
				utils.ManagedGo(func() {
					s.Lock()
					defer s.Unlock()
					for i, p := range s.pws {
						if p.Plan.ExecutionID == executionID {
							succeeded := []motion.PlanStatus{{State: motion.PlanStateSucceeded}}
							p.StatusHistory = append(succeeded, p.StatusHistory...)
							s.pws[i] = p
							return
						}
					}
					t.Error("MoveOnGlobeNew called unexpectedly")
				}, wg.Done)
			}
			return executionID.String(), nil
		}

		planHistoryCalledCtx, planHistoryCalledCancelFn := context.WithTimeout(ctx, time.Millisecond*500)
		defer planHistoryCalledCancelFn()
		s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			s.RLock()
			defer s.RUnlock()
			history := make([]motion.PlanWithStatus, len(s.pws))
			copy(history, s.pws)
			planHistoryCalledCancelFn()
			return history, nil
		}

		s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
			s.Lock()
			defer s.Unlock()
			if s.sprs == nil {
				s.sprs = []motion.StopPlanReq{}
			}
			s.sprs = append(s.sprs, req)
			return nil
		}

		// Set manual mode to ensure waypoint loop from prior test exits
		points := []*geo.Point{geo.NewPoint(1, 2), geo.NewPoint(2, 3)}
		for _, pt := range points {
			err := s.ns.AddWaypoint(ctx, pt, nil)
			test.That(t, err, test.ShouldBeNil)
		}

		wps, err := s.ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		wp1 := wps[0]
		// start navigation - set ModeManual first to ensure navigation starts up
		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)

		// wait for plan history to be called, indicating that MoveOnGlobe is in progress
		<-planHistoryCalledCtx.Done()

		err = s.ns.RemoveWaypoint(ctx, wp1.ID, nil)
		test.That(t, err, test.ShouldBeNil)

		// Reach the second waypoint
		timeoutCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*1000)
		defer cancelFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Error("test timed out")
				t.FailNow()
			}
			// once all waypoints have been consumed
			wps, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if len(wps) == 0 {
				s.RLock()
				// // MoveOnGlobeNew was called twice, once for each waypoint
				test.That(t, len(s.mogrs), test.ShouldEqual, 2)
				// // waypoint 1
				test.That(t, s.mogrs[0].Destination, test.ShouldResemble, points[0])
				// // waypoint 2
				test.That(t, s.mogrs[1].Destination, test.ShouldResemble, points[1])
				// Motion reports that the last execution succeeded
				s.RUnlock()

				ph, err := s.injectMS.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: s.base.Name()})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(ph), test.ShouldEqual, 1)
				test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionIDs[1])
				test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 2)
				test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
				break
			}
		}
	})

	t.Run("Calling RemoveWaypoint on a waypoint that is not in progress does not cancel MoveOnGlobe", func(t *testing.T) {
		s := setupStartWaypoint(ctx, t, logger)
		defer s.closeFunc()

		// we expect 2 executions to be generated
		executionIDs := []uuid.UUID{
			uuid.New(),
			uuid.New(),
		}
		counter := atomic.NewInt32(-1)
		var wg sync.WaitGroup
		defer wg.Wait()
		pauseMOGSuccess := make(chan struct{})
		resumeMOGSuccess := make(chan struct{})
		// MoveOnGlobeNew will behave as if it created a new plan & queue up a goroutine which will then behave as if the plan succeeded
		s.injectMS.MoveOnGlobeNewFunc = func(ctx context.Context, req motion.MoveOnGlobeReq) (string, error) {
			executionID := executionIDs[(counter.Inc())]
			s.Lock()
			defer s.Unlock()
			if s.mogrs == nil {
				s.mogrs = []motion.MoveOnGlobeReq{}
			}
			s.mogrs = append(s.mogrs, req)
			s.pws = []motion.PlanWithStatus{
				{
					Plan: motion.Plan{
						ExecutionID: executionID,
					},
					StatusHistory: []motion.PlanStatus{
						{State: motion.PlanStateInProgress},
					},
				},
			}
			wg.Add(1)
			utils.ManagedGo(func() {
				pauseMOGSuccess <- struct{}{}
				<-resumeMOGSuccess
				s.Lock()
				defer s.Unlock()
				for i, p := range s.pws {
					if p.Plan.ExecutionID == executionID {
						succeeded := []motion.PlanStatus{{State: motion.PlanStateSucceeded}}
						p.StatusHistory = append(succeeded, p.StatusHistory...)
						s.pws[i] = p
						return
					}
				}
				t.Error("MoveOnGlobeNew called unexpectedly")
			}, wg.Done)
			return executionID.String(), nil
		}

		s.injectMS.PlanHistoryFunc = func(ctx context.Context, req motion.PlanHistoryReq) ([]motion.PlanWithStatus, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			s.RLock()
			defer s.RUnlock()
			history := make([]motion.PlanWithStatus, len(s.pws))
			copy(history, s.pws)
			return history, nil
		}

		s.injectMS.StopPlanFunc = func(ctx context.Context, req motion.StopPlanReq) error {
			s.Lock()
			defer s.Unlock()
			if s.sprs == nil {
				s.sprs = []motion.StopPlanReq{}
			}
			s.sprs = append(s.sprs, req)
			return nil
		}

		// Set manual mode to ensure waypoint loop from prior test exits
		points := []*geo.Point{geo.NewPoint(1, 2), geo.NewPoint(2, 3)}
		for _, pt := range points {
			err := s.ns.AddWaypoint(ctx, pt, nil)
			test.That(t, err, test.ShouldBeNil)
		}

		wps, err := s.ns.Waypoints(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		wp2 := wps[1]
		// start navigation - set ModeManual first to ensure navigation starts up
		err = s.ns.SetMode(ctx, navigation.ModeWaypoint, nil)
		test.That(t, err, test.ShouldBeNil)
		<-pauseMOGSuccess

		err = s.ns.RemoveWaypoint(ctx, wp2.ID, nil)
		test.That(t, err, test.ShouldBeNil)
		resumeMOGSuccess <- struct{}{}

		// Reach the second waypoint
		timeoutCtx, cancelFn := context.WithTimeout(ctx, time.Millisecond*1000)
		defer cancelFn()
		for {
			if timeoutCtx.Err() != nil {
				t.Error("test timed out")
				t.FailNow()
			}
			// once all waypoints have been consumed
			wps, err := s.ns.Waypoints(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			if len(wps) == 0 {
				s.RLock()
				// // MoveOnGlobeNew was called once with waypoint 1
				test.That(t, len(s.mogrs), test.ShouldEqual, 1)
				// // waypoint 1
				test.That(t, s.mogrs[0].Destination, test.ShouldResemble, points[0])
				// Motion reports that the last execution succeeded
				s.RUnlock()

				ph, err := s.injectMS.PlanHistory(ctx, motion.PlanHistoryReq{ComponentName: s.base.Name()})
				test.That(t, err, test.ShouldBeNil)
				test.That(t, len(ph), test.ShouldEqual, 1)
				test.That(t, ph[0].Plan.ExecutionID, test.ShouldResemble, executionIDs[0])
				test.That(t, len(ph[0].StatusHistory), test.ShouldEqual, 2)
				test.That(t, ph[0].StatusHistory[0].State, test.ShouldEqual, motion.PlanStateSucceeded)
				break
			}
		}
	})
}

func TestValidateGeometry(t *testing.T) {
	cfg := Config{
		BaseName:           "base",
		MapType:            "GPS",
		MovementSensorName: "localizer",
		ObstacleDetectors: []*ObstacleDetectorNameConfig{
			{
				VisionServiceName: "vision",
				CameraName:        "camera",
			},
		},
	}

	createBox := func(translation r3.Vector) Config {
		boxPose := spatialmath.NewPoseFromPoint(translation)
		geometries, err := spatialmath.NewBox(boxPose, r3.Vector{X: 10, Y: 10, Z: 10}, "")
		test.That(t, err, test.ShouldBeNil)

		geoObstacle := spatialmath.NewGeoObstacle(geo.NewPoint(0, 0), []spatialmath.Geometry{geometries})
		geoObstacleCfg, err := spatialmath.NewGeoObstacleConfig(geoObstacle)
		test.That(t, err, test.ShouldBeNil)

		cfg.Obstacles = []*spatialmath.GeoObstacleConfig{geoObstacleCfg}

		return cfg
	}

	t.Run("fail case", func(t *testing.T) {
		cfg = createBox(r3.Vector{X: 10, Y: 10, Z: 10})
		_, err := cfg.Validate("")
		expectedErr := "geometries specified through the navigation are not allowed to have a translation"
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, expectedErr)
	})

	t.Run("success case", func(t *testing.T) {
		cfg = createBox(r3.Vector{})
		_, err := cfg.Validate("")
		test.That(t, err, test.ShouldBeNil)
	})
}
