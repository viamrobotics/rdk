// Package builtin implements a navigation service.
package builtin

import (
	"context"
	"encoding/json"
	"math"
	"sync"
	"sync/atomic"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	defaultLinearVelocityMPerSec     = 0.5
	defaultAngularVelocityDegsPerSec = 45

	// how far off the path must the robot be to trigger replanning.
	defaultPlanDeviationM = 1e9

	// the allowable quality change between the new plan and the remainder
	// of the original plan.
	defaultReplanCostFactor = 1.

	// frequency measured in hertz.
	defaultObstaclePollingFrequencyHz = 2.
	defaultPositionPollingFrequencyHz = 2.
)

var errClosed error = errors.New("already closed")

func init() {
	resource.RegisterService(navigation.API, resource.DefaultServiceModel, resource.Registration[navigation.Service, *Config]{
		Constructor: NewBuiltIn,
		// TODO: We can move away from using AttributeMapConverter if we change the way
		// that we allow orientations to be specified within orientation_json.go
		AttributeMapConverter: func(attributes rdkutils.AttributeMap) (*Config, error) {
			b, err := json.Marshal(attributes)
			if err != nil {
				return nil, err
			}

			var cfg Config
			if err := json.Unmarshal(b, &cfg); err != nil {
				return nil, err
			}
			return &cfg, nil
		},
	})
}

// Config describes how to configure the service.
type Config struct {
	Store              navigation.StoreConfig `json:"store"`
	BaseName           string                 `json:"base"`
	MovementSensorName string                 `json:"movement_sensor"`
	MotionServiceName  string                 `json:"motion_service"`
	VisionServices     []string               `json:"vision_services"`

	// DegPerSec and MetersPerSec are targets and not hard limits on speed
	DegPerSec    float64 `json:"degs_per_sec,omitempty"`
	MetersPerSec float64 `json:"meters_per_sec,omitempty"`

	Obstacles                  []*spatialmath.GeoObstacleConfig `json:"obstacles,omitempty"`
	PositionPollingFrequencyHz float64                          `json:"position_polling_frequency_hz,omitempty"`
	ObstaclePollingFrequencyHz float64                          `json:"obstacle_polling_frequency_hz,omitempty"`
	PlanDeviationM             float64                          `json:"plan_deviation_m,omitempty"`
	ReplanCostFactor           float64                          `json:"replan_cost_factor,omitempty"`
	LogFilePath                string                           `json:"log_file_path"`
}

// Validate creates the list of implicit dependencies.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	if conf.BaseName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	deps = append(deps, conf.BaseName)

	if conf.MovementSensorName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}
	deps = append(deps, conf.MovementSensorName)

	if conf.MotionServiceName == "" {
		conf.MotionServiceName = "builtin"
	}
	deps = append(deps, resource.NewName(motion.API, conf.MotionServiceName).String())

	for _, v := range conf.VisionServices {
		deps = append(deps, resource.NewName(vision.API, v).String())
	}

	// get default speeds from config if set, else defaults from nav services const
	if conf.MetersPerSec == 0 {
		conf.MetersPerSec = defaultLinearVelocityMPerSec
	}
	if conf.DegPerSec == 0 {
		conf.DegPerSec = defaultAngularVelocityDegsPerSec
	}
	if conf.PositionPollingFrequencyHz == 0 {
		conf.PositionPollingFrequencyHz = defaultPositionPollingFrequencyHz
	}
	if conf.ObstaclePollingFrequencyHz == 0 {
		conf.ObstaclePollingFrequencyHz = defaultObstaclePollingFrequencyHz
	}
	if conf.PlanDeviationM == 0 {
		conf.PlanDeviationM = defaultPlanDeviationM
	}
	if conf.ReplanCostFactor == 0 {
		conf.ReplanCostFactor = defaultReplanCostFactor
	}

	// ensure obstacles have no translation
	for _, obs := range conf.Obstacles {
		for _, geoms := range obs.Geometries {
			if !geoms.TranslationOffset.ApproxEqual(r3.Vector{}) {
				return nil, errors.New("geometries specified through the navigation are not allowed to have a translation")
			}
		}
	}

	return deps, nil
}

// NewBuiltIn returns a new navigation service for the given robot.
func NewBuiltIn(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (navigation.Service, error) {
	navSvc := &builtIn{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := navSvc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}

	return navSvc, nil
}

type waypointModeState struct {
	// protects waypointModeState struct
	mu                        sync.RWMutex
	cancelFunc                func()
	currentWaypointCancelFunc func()
	currentWaypoint           *navigation.Waypoint
	modeWorkers               sync.WaitGroup
}

type builtIn struct {
	resource.Named
	// protects service level lifecycle methods
	serviceMu sync.RWMutex
	// set to true if the service is closed, methods should return errors if closed
	closed atomic.Bool

	store            navigation.NavStore
	storeType        string
	base             base.Base
	movementSensor   movementsensor.MovementSensor
	motion           motion.Service
	obstacles        []*spatialmath.GeoObstacle
	motionCfg        *motion.MotionConfiguration
	replanCostFactor float64

	logger golog.Logger

	// protects mode
	modeMu  sync.RWMutex
	mode    navigation.Mode
	wmState waypointModeState
}

func (svc *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.serviceMu.Lock()
	defer svc.serviceMu.Unlock()

	svc.wmState.stop()
	svc.mode = navigation.ModeManual

	svcConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	if svcConfig.LogFilePath != "" {
		logger, err := rdkutils.NewFilePathDebugLogger(svcConfig.LogFilePath, "navigation")
		if err != nil {
			return err
		}
		svc.logger = logger
	}
	base, err := base.FromDependencies(deps, svcConfig.BaseName)
	if err != nil {
		return err
	}
	movementSensor, err := movementsensor.FromDependencies(deps, svcConfig.MovementSensorName)
	if err != nil {
		return err
	}
	motionSvc, err := motion.FromDependencies(deps, svcConfig.MotionServiceName)
	if err != nil {
		return err
	}

	var visionServices []resource.Name
	for _, svc := range svcConfig.VisionServices {
		visionSvc, err := vision.FromDependencies(deps, svc)
		if err != nil {
			return err
		}
		visionServices = append(visionServices, visionSvc.Name())
	}

	var newStore navigation.NavStore
	if svc.storeType != string(svcConfig.Store.Type) {
		switch svcConfig.Store.Type {
		case navigation.StoreTypeMemory:
			newStore = navigation.NewMemoryNavigationStore()
		case navigation.StoreTypeMongoDB:
			var err error
			newStore, err = navigation.NewMongoDBNavigationStore(ctx, svcConfig.Store.Config)
			if err != nil {
				return err
			}
		default:
			return errors.Errorf("unknown store type %q", svcConfig.Store.Type)
		}
	} else {
		newStore = svc.store
	}

	// Parse obstacles from the passed in configuration
	newObstacles, err := spatialmath.GeoObstaclesFromConfigs(svcConfig.Obstacles)
	if err != nil {
		return err
	}

	svc.store = newStore
	svc.storeType = string(svcConfig.Store.Type)
	svc.base = base
	svc.movementSensor = movementSensor
	svc.motion = motionSvc
	svc.obstacles = newObstacles
	svc.replanCostFactor = svcConfig.ReplanCostFactor
	svc.motionCfg = &motion.MotionConfiguration{
		VisionServices:        visionServices,
		LinearMPerSec:         svcConfig.MetersPerSec,
		AngularDegsPerSec:     svcConfig.DegPerSec,
		PlanDeviationMM:       1e3 * svcConfig.PlanDeviationM,
		PositionPollingFreqHz: svcConfig.PositionPollingFrequencyHz,
		ObstaclePollingFreqHz: svcConfig.ObstaclePollingFrequencyHz,
	}
	svc.closed.Store(false)

	return nil
}

func (svc *builtIn) Mode(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
	if svc.closed.Load() {
		return 0, errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	svc.modeMu.RLock()
	defer svc.modeMu.RUnlock()
	return svc.mode, nil
}

func (svc *builtIn) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	if svc.closed.Load() {
		return errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	svc.modeMu.RLock()
	svc.logger.Infof("SetMode called with mode: %s, transitioning to mode: %s", mode, svc.mode)
	if svc.mode == mode {
		svc.modeMu.RUnlock()
		return nil
	}
	svc.modeMu.RUnlock()

	// switch modes
	svc.modeMu.Lock()
	svc.wmState.stop()
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	svc.wmState.cancelFunc = cancelFunc
	svc.mode = mode
	svc.modeMu.Unlock()
	if svc.mode == navigation.ModeWaypoint {
		svc.startWaypointMode(cancelCtx, extra)
	}
	return nil
}

func (svc *builtIn) Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
	if svc.closed.Load() {
		return nil, errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	if svc.movementSensor == nil {
		return nil, errors.New("no way to get location")
	}
	loc, _, err := svc.movementSensor.Position(ctx, extra)
	if err != nil {
		return nil, err
	}
	compassHeading, err := svc.movementSensor.CompassHeading(ctx, extra)
	if err != nil {
		return nil, err
	}
	geoPose := spatialmath.NewGeoPose(loc, compassHeading)
	return geoPose, err
}

func (svc *builtIn) Waypoints(ctx context.Context, extra map[string]interface{}) ([]navigation.Waypoint, error) {
	if svc.closed.Load() {
		return nil, errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	wps, err := svc.store.Waypoints(ctx)
	if err != nil {
		return nil, err
	}
	wpsCopy := make([]navigation.Waypoint, 0, len(wps))
	wpsCopy = append(wpsCopy, wps...)
	return wpsCopy, nil
}

func (svc *builtIn) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	if svc.closed.Load() {
		return errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	svc.logger.Infof("AddWaypoint called with %#v", *point)
	_, err := svc.store.AddWaypoint(ctx, point)
	return err
}

func (svc *builtIn) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	if svc.closed.Load() {
		return errClosed
	}

	svc.serviceMu.RLock()
	defer svc.serviceMu.RUnlock()

	svc.logger.Infof("RemoveWaypoint called with waypointID: %s", id)
	svc.wmState.cancelWaypoint(id)
	return svc.store.RemoveWaypoint(ctx, id)
}

func (svc *builtIn) Close(ctx context.Context) error {
	svc.serviceMu.Lock()
	defer svc.serviceMu.Unlock()
	svc.closed.Store(true)

	svc.wmState.stop()
	return svc.store.Close(ctx)
}

func (svc *builtIn) GetObstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
	if svc.closed.Load() {
		return nil, errClosed
	}

	svc.serviceMu.Lock()
	defer svc.serviceMu.Unlock()

	return svc.obstacles, nil
}

func (svc *builtIn) startWaypointMode(ctx context.Context, extra map[string]interface{}) {
	svc.logger.Debug("startWaypointMode called")
	if extra == nil {
		if false {
			extra = map[string]interface{}{"motion_profile": "position_only"}
		} // TODO: Fix with RSDK-4583
	} else if _, ok := extra["motion_profile"]; !ok {
		if false {
			extra["motion_profile"] = "position_only"
		} // TODO: Fix with RSDK-4583
	}

	svc.wmState.modeWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.wmState.modeWorkers.Done()

		navOnce := func(ctx context.Context, wp navigation.Waypoint) error {
			svc.logger.Debugf("MoveOnGlobe called going to waypoint %+v", wp)
			success, err := svc.motion.MoveOnGlobe(
				ctx,
				svc.base.Name(),
				wp.ToPoint(),
				math.NaN(),
				svc.movementSensor.Name(),
				svc.obstacles,
				svc.motionCfg,
				extra,
			)
			if err != nil {
				err = errors.Wrapf(err, "hit motion error when navigating to waypoint %+v", wp)
				return err
			}

			if !success {
				err := errors.New("failed to reach goal")
				err = errors.Wrapf(err, "hit motion error when navigating to waypoint %+v", wp)
				return err
			}
			svc.logger.Debug("MoveOnGlobe succeeded")
			return nil
		}

		// do not exit loop - even if there are no waypoints remaining
		for {
			if ctx.Err() != nil {
				return
			}

			wp, err := svc.store.NextWaypoint(ctx)
			if err != nil {
				continue
			}

			cancelCtx, err := svc.wmState.setWaypoint(ctx, &wp)
			if err != nil {
				svc.logger.Error(err)
				if closeErr := svc.Close(ctx); closeErr != nil {
					svc.logger.Error(errors.Wrapf(err, "navigation Close() returned an error %s", closeErr))
				}
				return
			}

			svc.logger.Infof("navigating to waypoint: %+v", wp)
			if err := navOnce(cancelCtx, wp); err != nil {
				if svc.wmState.current() == nil {
					svc.logger.Debug(errors.Wrapf(err, "skipping waypoint %+v since it was deleted", wp))
				} else {
					svc.logger.Warn(err)
				}
				continue
			}

			// mark the waypoint as reached if the motion service hit no error
			if err = svc.store.WaypointVisited(ctx, wp.ID); err != nil {
				// If waypointReached returned an error it indicates the store didn't
				// record the waypoint as being reached, so we Close the nav service
				// as otherwise we will try to continue navigating to a waypoint
				// we have already reached.
				err = errors.Wrapf(err, "failed to update the waypoint db for waypoint %+v, calling Close() on nav", wp)
				svc.logger.Error(err)
				if closeErr := svc.Close(ctx); closeErr != nil {
					svc.logger.Error(errors.Wrapf(err, "navigation Close() returned an error %s", closeErr))
				}
				return
			}
			svc.logger.Infof("reached waypoint %s", wp.ID)
		}
	})
}

func (wpmState *waypointModeState) stop() {
	wpmState.mu.Lock()
	defer wpmState.mu.Unlock()
	if wpmState.cancelFunc != nil {
		wpmState.cancelFunc()
	}
	wpmState.modeWorkers.Wait()
	wpmState.currentWaypoint = nil
}

func (wpmState *waypointModeState) current() *navigation.Waypoint {
	wpmState.mu.RLock()
	defer wpmState.mu.RUnlock()
	return wpmState.currentWaypoint
}

func (wpmState *waypointModeState) cancelWaypoint(id primitive.ObjectID) {
	wpmState.mu.Lock()
	if wpmState.currentWaypoint != nil && wpmState.currentWaypoint.ID == id {
		if wpmState.currentWaypointCancelFunc != nil {
			wpmState.currentWaypointCancelFunc()
		}
		wpmState.currentWaypoint = nil
	}
	wpmState.mu.Unlock()
}

func (wpmState *waypointModeState) setWaypoint(ctx context.Context, wp *navigation.Waypoint) (context.Context, error) {
	wpmState.mu.Lock()
	defer wpmState.mu.Unlock()
	if wp == nil {
		return context.Background(), errors.New("can't set current waypoint to nil")
	}
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	wpmState.currentWaypointCancelFunc = cancelFunc
	wpmState.currentWaypoint = wp
	return cancelCtx, nil
}
