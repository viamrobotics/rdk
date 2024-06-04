// Package builtin implements a navigation service.
package builtin

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
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
	geomWithTranslation = "geometries specified through navigation are not allowed to have a translation"

	errNegativeDegPerSec                  = errors.New("degs_per_sec must be non-negative if set")
	errNegativeMetersPerSec               = errors.New("meters_per_sec must be non-negative if set")
	errNegativePositionPollingFrequencyHz = errors.New("position_polling_frequency_hz must be non-negative if set")
	errNegativeObstaclePollingFrequencyHz = errors.New("obstacle_polling_frequency_hz must be non-negative if set")
	errNegativePlanDeviationM             = errors.New("plan_deviation_m must be non-negative if set")
	errNegativeReplanCostFactor           = errors.New("replan_cost_factor must be non-negative if set")
	errObstacleGeomWithTranslation        = errors.New("obstacle " + geomWithTranslation)
	errBoundingRegionsGeomWithTranslation = errors.New("bounding region " + geomWithTranslation)
	errObstacleGeomParse                  = errors.New("obstacle unable to be converted from geometry config")
	errBoundingRegionsGeomParse           = errors.New("bounding regions unable to be converted from geometry config")
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
	defaultObstaclePollingHz = 1.
	defaultPositionPollingHz = 1.

	// frequency in milliseconds.
	planHistoryPollFrequency = time.Millisecond * 50
)

func init() {
	resource.RegisterService(navigation.API, resource.DefaultServiceModel, resource.Registration[navigation.Service, *Config]{
		Constructor: NewBuiltIn,
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

	Obstacles                  []*spatialmath.GeoGeometryConfig `json:"obstacles,omitempty"`
	BoundingRegions            []*spatialmath.GeoGeometryConfig `json:"bounding_regions,omitempty"`
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
				return nil, errObstacleGeomWithTranslation
			}
		}
	}

	// Ensure bounding regions have no translation
	for _, region := range conf.BoundingRegions {
		for _, geoms := range region.Geometries {
			if !geoms.TranslationOffset.ApproxEqual(r3.Vector{}) {
				return nil, errBoundingRegionsGeomWithTranslation
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

	fsService            framesystem.Service
	base                 base.Base
	movementSensor       movementsensor.MovementSensor
	visionServicesByName map[resource.Name]vision.Service
	motionService        motion.Service
	// exploreMotionService will be removed once the motion explore model is integrated into motion builtin
	exploreMotionService motion.Service
	obstacles            []*spatialmath.GeoGeometry
	boundingRegions      []*spatialmath.GeoGeometry

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

	// Set framesystem service
	for name, dep := range deps {
		if name == framesystem.InternalServiceName {
			fsService, ok := dep.(framesystem.Service)
			if !ok {
				return errors.New("frame system service is invalid type")
			}
			svc.fsService = fsService
			break
		}
	}

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
	visionServicesByName := make(map[resource.Name]vision.Service)
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
		visionServicesByName[visionSvc.Name()] = visionSvc
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
	newObstacles, err := spatialmath.GeoGeometriesFromConfigs(svcConfig.Obstacles)
	if err != nil {
		return errors.Wrap(errObstacleGeomParse, err.Error())
	}

	// Parse bounding regions from the configuration
	newBoundingRegions, err := spatialmath.GeoGeometriesFromConfigs(svcConfig.BoundingRegions)
	if err != nil {
		return errors.Wrap(errBoundingRegionsGeomParse, err.Error())
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
	svc.boundingRegions = newBoundingRegions
	svc.replanCostFactor = replanCostFactor
	svc.visionServicesByName = visionServicesByName
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
	svc.logger.CInfof(ctx, "SetMode called: transitioning from %s to %s", svc.mode, mode)
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
	movementsensorOrigin := referenceframe.NewPoseInFrame(svc.movementSensor.Name().ShortName(), spatialmath.NewZeroPose())
	movementSensorPoseInBase, err := svc.fsService.TransformPose(ctx, movementsensorOrigin, svc.base.Name().ShortName(), nil)
	if err != nil {
		// here we make the assumption the movementsensor is coincident with the camera
		svc.logger.CDebugf(
			ctx,
			"we assume the movementsensor named: %s is coincident with the base named: %s due to err: %v",
			svc.movementSensor.Name().ShortName(), svc.base.Name(), err.Error(),
		)
		movementSensorPoseInBase = referenceframe.NewPoseInFrame(svc.base.Name().ShortName(), spatialmath.NewZeroPose())
	}
	svc.logger.CDebugf(ctx, "movementSensorPoseInBase: %v", spatialmath.PoseToProtobuf(movementSensorPoseInBase.Pose()))

	movementSensor2dOrientation, err := spatialmath.ProjectOrientationTo2dRotation(movementSensorPoseInBase.Pose())
	if err != nil {
		return nil, err
	}
	// When rotation about the +Z axis, an OV theta is right handed but compass heading is left handed. Account for this.
	compassHeading -= movementSensor2dOrientation.Orientation().OrientationVectorDegrees().Theta
	if compassHeading < 0 {
		compassHeading += 360
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
	svc.logger.CInfof(ctx, "AddWaypoint called with %#v", *point)
	_, err := svc.store.AddWaypoint(ctx, point)
	return err
}

func (svc *builtIn) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.logger.CInfof(ctx, "RemoveWaypoint called with waypointID: %s", id)
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
		BoundingRegions:    svc.boundingRegions,
		Extra:              extra,
	}
	cancelCtx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()
	executionID, err := svc.motionService.MoveOnGlobe(cancelCtx, req)
	if errors.Is(err, motion.ErrGoalWithinPlanDeviation) {
		// make an exception for the error that is raised when motion is not possible because already at goal.
		return svc.waypointReached(cancelCtx)
	} else if err != nil {
		return err
	}

	executionWaypoint := executionWaypoint{executionID: executionID, waypoint: wp}
	if old := svc.activeExecutionWaypoint.Swap(executionWaypoint); old != nil && old != emptyExecutionWaypoint {
		msg := "unexpected race condition in moveOnGlobeSync, expected " +
			"replaced waypoint & execution id to be nil or %#v; instead was %s"
		svc.logger.CErrorf(ctx, msg, emptyExecutionWaypoint, old)
	}
	// call StopPlan upon exiting moveOnGlobeSync
	// is a NoOp if execution has already terminted
	defer func() {
		timeoutCtx, timeoutCancelFn := context.WithTimeout(context.Background(), time.Second*5)
		defer timeoutCancelFn()
		err := svc.motionService.StopPlan(timeoutCtx, motion.StopPlanReq{ComponentName: req.ComponentName})
		if err != nil {
			svc.logger.CError(ctx, "hit error trying to stop plan %s", err)
		}

		if old := svc.activeExecutionWaypoint.Swap(emptyExecutionWaypoint); old != executionWaypoint {
			msg := "unexpected race condition in moveOnGlobeSync, expected " +
				"replaced waypoint & execution id to equal %s, was actually %s"
			svc.logger.CErrorf(ctx, msg, executionWaypoint, old)
		}
	}()

	err = motion.PollHistoryUntilSuccessOrError(cancelCtx, svc.motionService, planHistoryPollFrequency,
		motion.PlanHistoryReq{
			ComponentName: req.ComponentName,
			ExecutionID:   executionID,
			LastPlanOnly:  true,
		},
	)
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

			svc.logger.CInfof(ctx, "navigating to waypoint: %+v", wp)
			if err := svc.moveToWaypoint(cancelCtx, wp, extra); err != nil {
				if svc.waypointIsDeleted() {
					svc.logger.CInfof(ctx, "skipping waypoint %+v since it was deleted", wp)
					continue
				}
				svc.logger.CWarnf(ctx, "retrying navigation to waypoint %+v since it errored out: %s", wp, err)
				continue
			}
			svc.logger.CInfof(ctx, "reached waypoint: %+v", wp)
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

func (svc *builtIn) Obstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoGeometry, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	// get static GeoGeometriess
	geoGeometries := svc.obstacles

	for _, detector := range svc.motionCfg.ObstacleDetectors {
		// get the vision service
		visSvc, ok := svc.visionServicesByName[detector.VisionServiceName]
		if !ok {
			return nil, fmt.Errorf("vision service with name: %s not found", detector.VisionServiceName)
		}

		svc.logger.CDebugf(
			ctx,
			"proceeding to get detections from vision service: %s with camera: %s",
			detector.VisionServiceName.ShortName(),
			detector.CameraName.ShortName(),
		)

		// get the detections
		detections, err := visSvc.GetObjectPointClouds(ctx, detector.CameraName.Name, nil)
		if err != nil {
			return nil, err
		}

		// determine transform from camera to movement sensor
		movementsensorOrigin := referenceframe.NewPoseInFrame(svc.movementSensor.Name().ShortName(), spatialmath.NewZeroPose())
		cameraToMovementsensor, err := svc.fsService.TransformPose(ctx, movementsensorOrigin, detector.CameraName.ShortName(), nil)
		if err != nil {
			// here we make the assumption the movementsensor is coincident with the camera
			svc.logger.CDebugf(
				ctx,
				"we assume the movementsensor named: %s is coincident with the camera named: %s due to err: %v",
				svc.movementSensor.Name().ShortName(), detector.CameraName.ShortName(), err.Error(),
			)
			cameraToMovementsensor = movementsensorOrigin
		}
		svc.logger.CDebugf(ctx, "cameraToMovementsensor Pose: %v", spatialmath.PoseToProtobuf(cameraToMovementsensor.Pose()))

		// determine transform from base to movement sensor
		baseToMovementSensor, err := svc.fsService.TransformPose(ctx, movementsensorOrigin, svc.base.Name().ShortName(), nil)
		if err != nil {
			// here we make the assumption the movementsensor is coincident with the base
			svc.logger.CDebugf(
				ctx,
				"we assume the movementsensor named: %s is coincident with the base named: %s due to err: %v",
				svc.movementSensor.Name().ShortName(), svc.base.Name().ShortName(), err.Error(),
			)
			baseToMovementSensor = movementsensorOrigin
		}
		svc.logger.CDebugf(ctx, "baseToMovementSensor Pose: %v", spatialmath.PoseToProtobuf(baseToMovementSensor.Pose()))

		// determine transform from base to camera
		cameraOrigin := referenceframe.NewPoseInFrame(detector.CameraName.ShortName(), spatialmath.NewZeroPose())
		baseToCamera, err := svc.fsService.TransformPose(ctx, cameraOrigin, svc.base.Name().ShortName(), nil)
		if err != nil {
			// here we make the assumption the base is coincident with the camera
			svc.logger.CDebugf(
				ctx,
				"we assume the base named: %s is coincident with the camera named: %s due to err: %v",
				svc.base.Name().ShortName(), detector.CameraName.ShortName(), err.Error(),
			)
			baseToCamera = cameraOrigin
		}
		svc.logger.CDebugf(ctx, "baseToCamera Pose: %v", spatialmath.PoseToProtobuf(baseToCamera.Pose()))

		// get current geo position of robot
		gp, _, err := svc.movementSensor.Position(ctx, nil)
		if err != nil {
			return nil, err
		}

		// instantiate a localizer and use it to get our current position
		localizer := motion.NewMovementSensorLocalizer(svc.movementSensor, gp, spatialmath.NewZeroPose())
		currentPIF, err := localizer.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}

		// convert orientation of currentPIF to be left handed
		localizerHeading := math.Mod(math.Abs(currentPIF.Pose().Orientation().OrientationVectorDegrees().Theta-360), 360)

		// ensure baseToMovementSensor orientation is positive
		localizerBaseThetaDiff := math.Mod(math.Abs(baseToMovementSensor.Pose().Orientation().OrientationVectorDegrees().Theta+360), 360)

		baseHeading := math.Mod(localizerHeading+localizerBaseThetaDiff, 360)

		// convert geo position into GeoPose
		robotGeoPose := spatialmath.NewGeoPose(gp, baseHeading)
		svc.logger.CDebugf(ctx, "robotGeoPose Location: %v, Heading: %v", *robotGeoPose.Location(), robotGeoPose.Heading())

		// iterate through all detections and construct a geoGeometry to append
		for i, detection := range detections {
			svc.logger.CInfof(
				ctx,
				"detection %d pose with respect to camera frame: %v",
				i, spatialmath.PoseToProtobuf(detection.Geometry.Pose()),
			)
			// the position of the detection in the camera coordinate frame if it were at the movementsensor's location
			desiredPoint := detection.Geometry.Pose().Point().Sub(cameraToMovementsensor.Pose().Point())

			desiredPose := spatialmath.NewPose(
				desiredPoint,
				detection.Geometry.Pose().Orientation(),
			)

			transformBy := spatialmath.PoseBetweenInverse(detection.Geometry.Pose(), desiredPose)

			// get the manipulated geometry
			manipulatedGeom := detection.Geometry.Transform(transformBy)
			svc.logger.CDebugf(
				ctx,
				"detection %d pose from movementsensor's position with camera frame coordinate axes: %v ",
				i, spatialmath.PoseToProtobuf(manipulatedGeom.Pose()),
			)

			// fix axes of geometry's pose such that it is in the cooordinate system of the base
			manipulatedGeom = manipulatedGeom.Transform(spatialmath.NewPoseFromOrientation(baseToCamera.Pose().Orientation()))
			svc.logger.CDebugf(
				ctx,
				"detection %d pose from movementsensor's position with base frame coordinate axes: %v ",
				i, spatialmath.PoseToProtobuf(manipulatedGeom.Pose()),
			)

			// get the geometry's lat & lng along with its heading with respect to north as a left handed value
			obstacleGeoPose := spatialmath.PoseToGeoPose(robotGeoPose, manipulatedGeom.Pose())
			svc.logger.CDebugf(
				ctx,
				"obstacleGeoPose Location: %v, Heading: %v",
				*obstacleGeoPose.Location(), obstacleGeoPose.Heading(),
			)

			// prefix the label of the geometry so we know it is transient and add extra info
			label := "transient_" + strconv.Itoa(i) + "_" + detector.CameraName.Name
			if detection.Geometry.Label() != "" {
				label += "_" + detection.Geometry.Label()
			}
			detection.Geometry.SetLabel(label)
			svc.logger.Debug(detection.Geometry)

			// determine the desired geometry pose
			desiredPose = spatialmath.NewPoseFromOrientation(detection.Geometry.Pose().Orientation())

			// calculate what we need to transform by
			transformBy = spatialmath.PoseBetweenInverse(detection.Geometry.Pose(), desiredPose)

			// set the geometry's pose to desiredPose
			manipulatedGeom = detection.Geometry.Transform(transformBy)

			// create the geo obstacle
			obstacle := spatialmath.NewGeoGeometry(obstacleGeoPose.Location(), []spatialmath.Geometry{manipulatedGeom})

			// add manipulatedGeom to list of geoGeometries we return
			geoGeometries = append(geoGeometries, obstacle)
		}
	}

	return geoGeometries, nil
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

	path := ph[0].Plan.Path()
	geoPoints := make([]*geo.Point, 0, len(path))
	poses, err := path.GetFramePoses(svc.base.Name().ShortName())
	if err != nil {
		return nil, err
	}
	for _, p := range poses {
		geoPoints = append(geoPoints, geo.NewPoint(p.Point().Y, p.Point().X))
	}
	navPath, err := navigation.NewPath(ewp.waypoint.ID, geoPoints)
	if err != nil {
		return nil, err
	}
	return []*navigation.Path{navPath}, nil
}

func (svc *builtIn) Properties(ctx context.Context) (navigation.Properties, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	prop := navigation.Properties{
		MapType: svc.mapType,
	}
	return prop, nil
}
