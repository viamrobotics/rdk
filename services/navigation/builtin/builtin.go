// Package builtin implements a navigation service.
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"
	"golang.org/x/exp/slices"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/framesystem"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/motion/explore"
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
	defaultLinearMPerSec     = 0.3
	defaultAngularDegsPerSec = 20.

	// how far off the path must the robot be to trigger replanning.
	defaultPlanDeviationM = 2.6

	// the allowable quality change between the new plan and the remainder
	// of the original plan.
	defaultReplanCostFactor = 1.

	// frequency measured in hertz.
	defaultSmoothIter        = 20
	defaultObstaclePollingHz = 1.
	defaultPositionPollingHz = 1.

	// frequency in milliseconds.
	planHistoryPollFrequency = time.Millisecond * 50
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

type executionWaypoint struct {
	executionID motion.ExecutionID
	waypoint    navigation.Waypoint
}

var emptyExecutionWaypoint = executionWaypoint{}

// Validate creates the list of implicit dependencies.
func (conf *Config) Validate(path string) ([]string, error) {
	var deps []string

	// Add base dependencies
	if conf.BaseName == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "base")
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
		return nil, resource.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}

	for _, obstacleDetectorPair := range conf.ObstacleDetectors {
		if obstacleDetectorPair.VisionServiceName == "" || obstacleDetectorPair.CameraName == "" {
			return nil, resource.NewConfigValidationError(path, errors.New("an obstacle detector is missing either a camera or vision service"))
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

	// add framesystem service as dependency to be used by builtin and explore motion service
	deps = append(deps, framesystem.InternalServiceName.String())

	return deps, nil
}

// NewBuiltIn returns a new navigation service for the given robot.
func NewBuiltIn(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (navigation.Service, error) {
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
	activeExecutionWaypoint atomic.Value
	actionMu                sync.RWMutex
	mu                      sync.RWMutex
	store                   navigation.NavStore
	storeType               string
	mode                    navigation.Mode
	mapType                 navigation.MapType

	base           base.Base
	movementSensor movementsensor.MovementSensor
	motionService  motion.Service
	// exploreMotionService will be removed once the motion explore model is integrated into motion builtin
	exploreMotionService motion.Service
	obstacles            []*spatialmath.GeoObstacle

	motionCfg        *motion.MotionConfiguration
	replanCostFactor float64

	logger                    logging.Logger
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
	metersPerSec := defaultLinearMPerSec
	if svcConfig.MetersPerSec != 0 {
		metersPerSec = svcConfig.MetersPerSec
	}
	degPerSec := defaultAngularDegsPerSec
	if svcConfig.DegPerSec != 0 {
		degPerSec = svcConfig.DegPerSec
	}
	positionPollingFrequencyHz := defaultPositionPollingHz
	if svcConfig.PositionPollingFrequencyHz != 0 {
		positionPollingFrequencyHz = svcConfig.PositionPollingFrequencyHz
	}
	obstaclePollingFrequencyHz := defaultObstaclePollingHz
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
	// Note: this service will disappear after the explore motion model is integrated into builtIn
	exploreMotionConf := resource.Config{ConvertedAttributes: &explore.Config{}}
	svc.exploreMotionService, err = explore.NewExplore(ctx, deps, exploreMotionConf, svc.logger)
	if err != nil {
		return err
	}

	svc.mode = navigation.ModeManual
	svc.base = baseComponent
	svc.mapType = mapType
	svc.motionService = motionSvc
	svc.obstacles = newObstacles
	svc.replanCostFactor = replanCostFactor
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
	svc.logger.Infof("SetMode called: transitioning from %s to %s", svc.mode, mode)
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
		if len(svc.motionCfg.ObstacleDetectors) == 0 {
			return errors.New("explore mode requires at least one vision service")
		}
		svc.startExploreMode(cancelCtx)
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
	if err := svc.exploreMotionService.Close(ctx); err != nil {
		return err
	}
	return svc.store.Close(ctx)
}

func (svc *builtIn) moveToWaypoint(ctx context.Context, wp navigation.Waypoint, extra map[string]interface{}) error {
	req := motion.MoveOnGlobeReq{
		ComponentName:      svc.base.Name(),
		Destination:        wp.ToPoint(),
		Heading:            math.NaN(),
		MovementSensorName: svc.movementSensor.Name(),
		Obstacles:          svc.obstacles,
		MotionCfg:          svc.motionCfg,
		Extra:              extra,
	}
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	executionID, err := svc.motionService.MoveOnGlobeNew(cancelCtx, req)
	if err != nil {
		return err
	}

	executionWaypoint := executionWaypoint{executionID: executionID, waypoint: wp}
	if old := svc.activeExecutionWaypoint.Swap(executionWaypoint); old != nil && old != emptyExecutionWaypoint {
		msg := "unexpected race condition in moveOnGlobeSync, expected " +
			"replaced waypoint & execution id to be nil or %#v; instead was %s"
		svc.logger.Errorf(msg, emptyExecutionWaypoint, old)
	}
	// call StopPlan upon exiting moveOnGlobeSync
	// is a NoOp if execution has already terminted
	defer func() {
		timeoutCtx, timeoutCancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer timeoutCancelFn()
		err := svc.motionService.StopPlan(timeoutCtx, motion.StopPlanReq{ComponentName: req.ComponentName})
		if err != nil {
			svc.logger.Error("hit error trying to stop plan %s", err)
		}

		if old := svc.activeExecutionWaypoint.Swap(emptyExecutionWaypoint); old != executionWaypoint {
			msg := "unexpected race condition in moveOnGlobeSync, expected " +
				"replaced waypoint & execution id to equal %s, was actually %s"
			svc.logger.Errorf(msg, executionWaypoint, old)
		}
	}()

	err = pollUntilMOGSuccessOrError(cancelCtx, svc.motionService, planHistoryPollFrequency,
		motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		})

	if err != nil {
		return err
	}

	return svc.waypointReached(cancelCtx)
}

func (svc *builtIn) startWaypointMode(ctx context.Context, extra map[string]interface{}) {
	if extra == nil {
		extra = map[string]interface{}{}
	}

	extra["motion_profile"] = "position_only"

	svc.activeBackgroundWorkers.Add(1)
	utils.ManagedGo(func() {
		// do not exit loop - even if there are no waypoints remaining
		for {
			if ctx.Err() != nil {
				return
			}

			wp, err := svc.store.NextWaypoint(ctx)
			if err != nil {
				time.Sleep(planHistoryPollFrequency)
				continue
			}
			svc.mu.Lock()
			svc.waypointInProgress = &wp
			cancelCtx, cancelFunc := context.WithCancel(ctx)
			svc.currentWaypointCancelFunc = cancelFunc
			svc.mu.Unlock()

			svc.logger.Infof("navigating to waypoint: %+v", wp)
			if err := svc.moveToWaypoint(cancelCtx, wp, extra); err != nil {
				if svc.waypointIsDeleted() {
					svc.logger.Infof("skipping waypoint %+v since it was deleted", wp)
					continue
				}
				svc.logger.Warnf("retrying navigation to waypoint %+v since it errored out: %s", wp, err)
				continue
			}
			svc.logger.Infof("reached waypoint: %+v", wp)
		}
	}, svc.activeBackgroundWorkers.Done)
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

func (svc *builtIn) Obstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.obstacles, nil
}

func (svc *builtIn) Paths(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	rawExecutionWaypoint := svc.activeExecutionWaypoint.Load()
	// If there is no execution, return empty paths
	if rawExecutionWaypoint == nil || rawExecutionWaypoint == emptyExecutionWaypoint {
		return []*navigation.Path{}, nil
	}

	ewp, ok := rawExecutionWaypoint.(executionWaypoint)
	if !ok {
		return nil, errors.New("execution corrupt")
	}

	ph, err := svc.motionService.PlanHistory(ctx, motion.PlanHistoryReq{
		ComponentName: svc.base.Name(),
		ExecutionID:   ewp.executionID,
		LastPlanOnly:  true,
	})
	if err != nil {
		return nil, err
	}

	geoPoints := make([]*geo.Point, 0, len(ph[0].Plan.Steps))
	for _, s := range ph[0].Plan.Steps {
		if len(s) > 1 {
			return nil, errors.New("multi component paths are unsupported")
		}
		var pose spatialmath.Pose
		for _, p := range s {
			pose = p
		}

		geoPoint := geo.NewPoint(pose.Point().Y, pose.Point().X)
		geoPoints = append(geoPoints, geoPoint)
	}

	path, err := navigation.NewPath(ewp.waypoint.ID, geoPoints)
	if err != nil {
		return nil, err
	}
	return []*navigation.Path{path}, nil
}

func pollUntilMOGSuccessOrError(
	ctx context.Context,
	m motion.Service,
	interval time.Duration,
	req motion.PlanHistoryReq,
) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		ph, err := m.PlanHistory(ctx, req)
		if err != nil {
			return err
		}

		status := ph[0].StatusHistory[0]

		switch status.State {
		case motion.PlanStateInProgress:
		case motion.PlanStateFailed:
			err := errors.New("plan failed")
			if reason := status.Reason; reason != nil {
				err = errors.Wrap(err, *reason)
			}
			return err

		case motion.PlanStateStopped:
			return errors.New("plan stopped")

		case motion.PlanStateSucceeded:
			return nil

		default:
			return fmt.Errorf("invalid plan state %d", status.State)
		}

		time.Sleep(interval)
	}
}
