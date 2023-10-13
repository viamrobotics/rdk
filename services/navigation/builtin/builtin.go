// Package builtin implements a navigation service.
package builtin

import (
	"context"
	"encoding/json"
	"math"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/explore"
	ex "go.viam.com/rdk/services/motion/explore"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

// available modes for each MapType.
var (
	availableModesByMapType = map[navigation.MapType][]navigation.Mode{
		navigation.NoMap:  {navigation.ModeManual, navigation.ModeExplore},
		navigation.GPSMap: {navigation.ModeManual, navigation.ModeWaypoint, navigation.ModeExplore},
	}

	errNegativeDegPerSec                  = errors.New("degs_per_sec must be non-negative if set")
	errNegativeMetersPerSec               = errors.New("meters_per_sec must be non-negative if set")
	errNegativePositionPollingFrequencyHz = errors.New("position_polling_frequency_hz must be non-negative if set")
	errNegativeObstaclePollingFrequencyHz = errors.New("obstacle_polling_frequency_hz must be non-negative if set")
	errNegativePlanDeviationM             = errors.New("plan_deviation_m must be non-negative if set")
	errNegativeReplanCostFactor           = errors.New("replan_cost_factor must be non-negative if set")
)

const (
	// default configuration for Store parameter.
	defaultStoreType = navigation.StoreTypeMemory

	// default map type is GPS.
	defaultMapType = navigation.GPSMap

	// desired speeds to maintain for the base.
	defaultLinearVelocityMPerSec     = 0.5
	defaultAngularVelocityDegsPerSec = 45.

	// how far off the path must the robot be to trigger replanning.
	defaultPlanDeviationM = 1e9

	// the allowable quality change between the new plan and the remainder
	// of the original plan.
	defaultReplanCostFactor = 1.

	// frequency measured in hertz.
	defaultObstaclePollingFrequencyHz = 2.
	defaultPositionPollingFrequencyHz = 2.
)

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

// ObstacleDetectorNameConfig is the protobuf version of ObstacleDetectorName.
type ObstacleDetectorNameConfig struct {
	VisionServiceName string `json:"vision_service"`
	CameraName        string `json:"camera"`
}

// ObstacleDetector pairs a vision service with a camera, informing the service about which camera it may use.
type ObstacleDetector struct {
	VisionService vision.Service
	Camera        camera.Camera
}

// Config describes how to configure the service.
type Config struct {
	Store              navigation.StoreConfig        `json:"store"`
	BaseName           string                        `json:"base"`
	MapType            string                        `json:"map_type"`
	MovementSensorName string                        `json:"movement_sensor"`
	MotionServiceName  string                        `json:"motion_service"`
	ObstacleDetectors  []*ObstacleDetectorNameConfig `json:"obstacle_detectors"`

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

	// Add base dependencies
	if conf.BaseName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	deps = append(deps, conf.BaseName)

	// Add movement sensor dependencies
	if conf.MovementSensorName != "" {
		deps = append(deps, conf.MovementSensorName)
	}

	// Add motion service dependencies
	if conf.MotionServiceName != "" {
		deps = append(deps, resource.NewName(motion.API, conf.MotionServiceName).String())
	} else {
		deps = append(deps, resource.NewName(motion.API, resource.DefaultServiceName).String())
	}

	// Ensure map_type is valid and a movement sensor is available if MapType is GPS (or default)
	mapType, err := navigation.StringToMapType(conf.MapType)
	if err != nil {
		return nil, err
	}
	if mapType == navigation.GPSMap && conf.MovementSensorName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}

	for _, obstacleDetectorPair := range conf.ObstacleDetectors {
		if obstacleDetectorPair.VisionServiceName == "" || obstacleDetectorPair.CameraName == "" {
			return nil, utils.NewConfigValidationError(path, errors.New("an obstacle detector is missing either a camera or vision service"))
		}
		deps = append(deps, resource.NewName(vision.API, obstacleDetectorPair.VisionServiceName).String())
		deps = append(deps, resource.NewName(camera.API, obstacleDetectorPair.CameraName).String())
	}

	// Ensure store is valid
	if err := conf.Store.Validate(path); err != nil {
		return nil, err
	}

	// Ensure inputs are non-negative
	if conf.DegPerSec < 0 {
		return nil, errNegativeDegPerSec
	}
	if conf.MetersPerSec < 0 {
		return nil, errNegativeMetersPerSec
	}
	if conf.PositionPollingFrequencyHz < 0 {
		return nil, errNegativePositionPollingFrequencyHz
	}
	if conf.ObstaclePollingFrequencyHz < 0 {
		return nil, errNegativeObstaclePollingFrequencyHz
	}
	if conf.PlanDeviationM < 0 {
		return nil, errNegativePlanDeviationM
	}
	if conf.ReplanCostFactor < 0 {
		return nil, errNegativeReplanCostFactor
	}

	// Ensure obstacles have no translation
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

type builtIn struct {
	resource.Named
	actionMu  sync.RWMutex
	mu        sync.RWMutex
	store     navigation.NavStore
	storeType string
	mode      navigation.Mode
	mapType   navigation.MapType

	base                 base.Base
	movementSensor       movementsensor.MovementSensor
	obstacleDetectors    []*ObstacleDetector
	motionService        motion.Service
	exploreMotionService motion.Service
	obstacles            []*spatialmath.GeoObstacle

	motionCfg        *motion.MotionConfiguration
	replanCostFactor float64

	logger                    golog.Logger
	wholeServiceCancelFunc    func()
	currentWaypointCancelFunc func()
	waypointInProgress        *navigation.Waypoint
	activeBackgroundWorkers   sync.WaitGroup
}

func (svc *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.actionMu.Lock()
	defer svc.actionMu.Unlock()

	svc.stopActiveMode()

	svcConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	// Set optional variables
	metersPerSec := defaultLinearVelocityMPerSec
	if svcConfig.MetersPerSec != 0 {
		metersPerSec = svcConfig.MetersPerSec
	}
	degPerSec := defaultAngularVelocityDegsPerSec
	if svcConfig.DegPerSec != 0 {
		degPerSec = svcConfig.DegPerSec
	}
	positionPollingFrequencyHz := defaultPositionPollingFrequencyHz
	if svcConfig.PositionPollingFrequencyHz != 0 {
		positionPollingFrequencyHz = svcConfig.PositionPollingFrequencyHz
	}
	obstaclePollingFrequencyHz := defaultObstaclePollingFrequencyHz
	if svcConfig.ObstaclePollingFrequencyHz != 0 {
		obstaclePollingFrequencyHz = svcConfig.ObstaclePollingFrequencyHz
	}
	planDeviationM := defaultPlanDeviationM
	if svcConfig.PlanDeviationM != 0 {
		planDeviationM = svcConfig.PlanDeviationM
	}
	replanCostFactor := defaultReplanCostFactor
	if svcConfig.ReplanCostFactor != 0 {
		replanCostFactor = svcConfig.ReplanCostFactor
	}

	motionServiceName := resource.DefaultServiceName
	if svcConfig.MotionServiceName != "" {
		motionServiceName = svcConfig.MotionServiceName
	}
	mapType := defaultMapType
	if svcConfig.MapType != "" {
		mapType, err = navigation.StringToMapType(svcConfig.MapType)
		if err != nil {
			return err
		}
	}

	storeCfg := navigation.StoreConfig{Type: defaultStoreType}
	if svcConfig.Store.Type != navigation.StoreTypeUnset {
		storeCfg = svcConfig.Store
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	// Parse logger file from the configuration if given
	if svcConfig.LogFilePath != "" {
		logger, err := rdkutils.NewFilePathDebugLogger(svcConfig.LogFilePath, "navigation")
		if err != nil {
			return err
		}
		svc.logger = logger
	}

	// Parse base from the configuration
	baseComponent, err := base.FromDependencies(deps, svcConfig.BaseName)
	if err != nil {
		return err
	}

	// Parse motion services from the configuration
	motionSvc, err := motion.FromDependencies(deps, motionServiceName)
	if err != nil {
		return err
	}

	var obstacleDetectorNamePairs []motion.ObstacleDetectorName
	var obstacleDetectors []*ObstacleDetector
	for _, pbObstacleDetectorPair := range svcConfig.ObstacleDetectors {
		visionSvc, err := vision.FromDependencies(deps, pbObstacleDetectorPair.VisionServiceName)
		if err != nil {
			return err
		}
		camera, err := camera.FromDependencies(deps, pbObstacleDetectorPair.CameraName)
		if err != nil {
			return err
		}
		obstacleDetectorNamePairs = append(obstacleDetectorNamePairs, motion.ObstacleDetectorName{
			VisionServiceName: visionSvc.Name(), CameraName: camera.Name(),
		})
		obstacleDetectors = append(obstacleDetectors, &ObstacleDetector{
			VisionService: visionSvc, Camera: camera,
		})
	}

	// Parse movement sensor from the configuration if map type is GPS
	if mapType == navigation.GPSMap {
		movementSensor, err := movementsensor.FromDependencies(deps, svcConfig.MovementSensorName)
		if err != nil {
			return err
		}
		svc.movementSensor = movementSensor
	}

	// Reconfigure the store if necessary
	if svc.storeType != string(storeCfg.Type) {
		newStore, err := navigation.NewStoreFromConfig(ctx, svcConfig.Store)
		if err != nil {
			return err
		}
		svc.store = newStore
		svc.storeType = string(storeCfg.Type)
	}

	// Parse obstacles from the configuration
	newObstacles, err := spatialmath.GeoObstaclesFromConfigs(svcConfig.Obstacles)
	if err != nil {
		return err
	}

	// Create explore motion service
	exploreMotionConf := resource.Config{ConvertedAttributes: &explore.Config{}}
	svc.exploreMotionService, err = ex.NewExplore(ctx, deps, exploreMotionConf, svc.logger)
	if err != nil {
		return err
	}

	svc.mode = navigation.ModeManual
	svc.base = baseComponent
	svc.mapType = mapType
	svc.motionService = motionSvc
	svc.obstacles = newObstacles
	svc.replanCostFactor = replanCostFactor
	svc.obstacleDetectors = obstacleDetectors
	svc.motionCfg = &motion.MotionConfiguration{
		ObstacleDetectors:     obstacleDetectorNamePairs,
		LinearMPerSec:         metersPerSec,
		AngularDegsPerSec:     degPerSec,
		PlanDeviationMM:       1e3 * planDeviationM,
		PositionPollingFreqHz: positionPollingFrequencyHz,
		ObstaclePollingFreqHz: obstaclePollingFrequencyHz,
	}

	return nil
}

func (svc *builtIn) Mode(ctx context.Context, extra map[string]interface{}) (navigation.Mode, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.mode, nil
}

func (svc *builtIn) SetMode(ctx context.Context, mode navigation.Mode, extra map[string]interface{}) error {
	svc.actionMu.Lock()
	defer svc.actionMu.Unlock()

	svc.mu.RLock()
	svc.logger.Infof("SetMode called with mode: %s, transitioning to mode: %s", mode, svc.mode)
	if svc.mode == mode {
		svc.mu.RUnlock()
		return nil
	}
	svc.mu.RUnlock()

	// stop passed active sessions
	svc.stopActiveMode()

	// switch modes
	svc.mu.Lock()
	defer svc.mu.Unlock()
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	svc.wholeServiceCancelFunc = cancelFunc
	svc.mode = mode

	if !slices.Contains(availableModesByMapType[svc.mapType], svc.mode) {
		return errors.Errorf("%v mode is unavailable for map type %v", svc.mode.String(), svc.mapType.String())
	}

	switch svc.mode {
	case navigation.ModeManual:
		// do nothing
	case navigation.ModeWaypoint:
		svc.startWaypointMode(cancelCtx, extra)
	case navigation.ModeExplore:
		if len(svc.obstacleDetectors) == 0 {
			return errors.New("explore mode requires at least one vision service")
		}
		svc.startExploreMode(cancelCtx)
		return errors.New("navigation mode 'explore' is not currently available")
	}

	return nil
}

func (svc *builtIn) Location(ctx context.Context, extra map[string]interface{}) (*spatialmath.GeoPose, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

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
	wps, err := svc.store.Waypoints(ctx)
	if err != nil {
		return nil, err
	}
	wpsCopy := make([]navigation.Waypoint, 0, len(wps))
	wpsCopy = append(wpsCopy, wps...)
	return wpsCopy, nil
}

func (svc *builtIn) AddWaypoint(ctx context.Context, point *geo.Point, extra map[string]interface{}) error {
	svc.logger.Infof("AddWaypoint called with %#v", *point)
	_, err := svc.store.AddWaypoint(ctx, point)
	return err
}

func (svc *builtIn) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.logger.Infof("RemoveWaypoint called with waypointID: %s", id)
	if svc.waypointInProgress != nil && svc.waypointInProgress.ID == id {
		if svc.currentWaypointCancelFunc != nil {
			svc.currentWaypointCancelFunc()
		}
		svc.waypointInProgress = nil
	}
	return svc.store.RemoveWaypoint(ctx, id)
}

func (svc *builtIn) waypointReached(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	svc.mu.RLock()
	wp := svc.waypointInProgress
	svc.mu.RUnlock()

	if wp == nil {
		return errors.New("can't mark waypoint reached since there is none in progress")
	}
	return svc.store.WaypointVisited(ctx, wp.ID)
}

func (svc *builtIn) Close(ctx context.Context) error {
	svc.actionMu.Lock()
	defer svc.actionMu.Unlock()

	svc.stopActiveMode()
	return svc.store.Close(ctx)
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

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		navOnce := func(ctx context.Context, wp navigation.Waypoint) error {
			svc.logger.Debugf("MoveOnGlobe called going to waypoint %+v", wp)
			_, err := svc.motionService.MoveOnGlobe(
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

			svc.logger.Debug("MoveOnGlobe succeeded")
			return svc.waypointReached(ctx)
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
			svc.mu.Lock()
			svc.waypointInProgress = &wp
			cancelCtx, cancelFunc := context.WithCancel(ctx)
			svc.currentWaypointCancelFunc = cancelFunc
			svc.mu.Unlock()

			svc.logger.Infof("navigating to waypoint: %+v", wp)
			if err := navOnce(cancelCtx, wp); err != nil {
				if svc.waypointIsDeleted() {
					svc.logger.Infof("skipping waypoint %+v since it was deleted", wp)
					continue
				}

				svc.logger.Infof("skipping waypoint %+v due to error while navigating towards it: %s", wp, err)
				if err := svc.waypointReached(ctx); err != nil {
					if svc.waypointIsDeleted() {
						svc.logger.Infof("skipping waypoint %+v since it was deleted", wp)
						continue
					}
					svc.logger.Infof("can't mark waypoint %+v as reached, exiting navigation due to error: %s", wp, err)
					return
				}
			}
		}
	})
}

func (svc *builtIn) stopActiveMode() {
	if svc.wholeServiceCancelFunc != nil {
		svc.wholeServiceCancelFunc()
	}
	svc.activeBackgroundWorkers.Wait()
}

func (svc *builtIn) waypointIsDeleted() bool {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.waypointInProgress == nil
}

func (svc *builtIn) GetObstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.obstacles, nil
}
