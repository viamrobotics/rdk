// Package builtin implements a navigation service.
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"

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

	errNegativeDegPerSec                  = errors.New("degs_per_sec must be non-negative if set")
	errNegativeMetersPerSec               = errors.New("meters_per_sec must be non-negative if set")
	errNegativePositionPollingFrequencyHz = errors.New("position_polling_frequency_hz must be non-negative if set")
	errNegativeObstaclePollingFrequencyHz = errors.New("obstacle_polling_frequency_hz must be non-negative if set")
	errNegativePlanDeviationM             = errors.New("plan_deviation_m must be non-negative if set")
	errNegativeReplanCostFactor           = errors.New("replan_cost_factor must be non-negative if set")
	errUnimplemented                      = errors.New("unimplemented")
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
	resource.RegisterService(
		navigation.API,
		resource.DefaultServiceModel,
		resource.Registration[navigation.Service, *Config]{
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
		},
	)
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
	fmt.Println("hello there 1")
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

	// add framesystem service as dependency
	deps = append(deps, framesystem.InternalServiceName.String())
	fmt.Println("hello there 2")

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
	fmt.Println("IN NEW BUILTIN FOR NAV")
	fmt.Println("ABOUT TO ENTER RECONFIG")
	if err := navSvc.Reconfigure(ctx, deps, conf); err != nil {
		fmt.Println("err: ", err.Error())
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

	fsService            framesystem.Service
	base                 base.Base
	movementSensor       movementsensor.MovementSensor
	visionServicesByName map[resource.Name]vision.Service
	motionService        motion.Service
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

	fmt.Println("hello there 3")

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
	fmt.Println("hello there 4")

	svc.logger.Info("WE ARE GOING TO TRY TO SET THIS UP")
	// create framesystem from dependencies
	svc.fsService, err = framesystem.New(ctx, deps, svc.logger)
	if err != nil {
		svc.logger.Info("FAILED TO SET UP FRAME SYSTEM SERVICE")
		svc.logger.Infof("ERR: %v", err.Error())
		return err
	}
	svc.logger.Info("SEEMS TO HAVE WORKED")

	svc.mode = navigation.ModeManual
	svc.base = baseComponent
	svc.mapType = mapType
	svc.motionService = motionSvc
	svc.obstacles = newObstacles
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
				return errors.Wrapf(err, "hit motion error when navigating to waypoint %+v", wp)
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
				svc.logger.Infof("retrying navigation to waypoint %+v since it errored out: %s", wp, err)
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

func (svc *builtIn) Obstacles(ctx context.Context, extra map[string]interface{}) ([]*spatialmath.GeoObstacle, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	// get static geoObstacles
	geoObstacles := svc.obstacles

	for _, detector := range svc.motionCfg.ObstacleDetectors {
		// get the vision service
		visSvc, ok := svc.visionServicesByName[detector.VisionServiceName]
		if !ok {
			return nil, fmt.Errorf("vision service with name: %s not found", detector.VisionServiceName)
		}

		// get the detections
		detections, err := visSvc.GetObjectPointClouds(ctx, detector.CameraName.Name, nil)
		if err != nil {
			return nil, err
		}

		// get transforms
		cameraToMovementsensor, baseToMovementSensor, baseToCamera, err := svc.getTransforms(ctx, detector.CameraName.ShortName())
		if err != nil {
			return nil, err
		}
		svc.logger.Infof("cameraToMovementsensor Pose.Point: %v", cameraToMovementsensor.Pose().Point())
		svc.logger.Infof("cameraToMovementsensor Pose: %v", spatialmath.PoseToProtobuf(cameraToMovementsensor.Pose()))

		svc.logger.Infof("baseToMovementSensor Pose.Point: %v", baseToMovementSensor.Pose().Point())
		svc.logger.Infof("baseToMovementSensor Pose: %v", spatialmath.PoseToProtobuf(baseToMovementSensor.Pose()))

		svc.logger.Infof("baseToCamera Pose.Point: %v", baseToCamera.Pose().Point())
		svc.logger.Infof("baseToCamera Pose: %v", spatialmath.PoseToProtobuf(baseToCamera.Pose()))

		// get current geo position of robot
		gp, _, err := svc.movementSensor.Position(ctx, nil)
		if err != nil {
			return nil, err
		}
		svc.logger.Infof("gp: %v", gp)

		// instantiate a localizer and use it to get our current position
		localizer := motion.NewMovementSensorLocalizer(svc.movementSensor, gp, spatialmath.NewZeroPose())
		currentPIF, err := localizer.CurrentPosition(ctx)
		if err != nil {
			return nil, err
		}

		// convert orientation of currentPIF to be left handed
		localizerHeading := math.Mod(math.Abs(currentPIF.Pose().Orientation().OrientationVectorDegrees().Theta-360), 360)
		svc.logger.Infof("localizerHeading: %v", localizerHeading)

		// convert orientation of movementsensorToCamera to be left handed????
		localizerBaseThetaDiff := math.Mod(math.Abs(baseToMovementSensor.Pose().Orientation().OrientationVectorDegrees().Theta+360), 360)
		svc.logger.Infof("localizerBaseThetaDiff: %v", localizerBaseThetaDiff)

		baseHeading := math.Mod(localizerHeading+localizerBaseThetaDiff, 360)
		svc.logger.Infof("baseHeading: %v", baseHeading)

		// convert geo position into GeoPose
		robotGeoPose := spatialmath.NewGeoPose(gp, baseHeading)
		svc.logger.Infof("robotGeoPose.Location(): %v", robotGeoPose.Location())
		svc.logger.Infof("robotGeoPose.Heading(): %v", robotGeoPose.Heading())

		// iterate through all detections and construct a geoObstacle to append
		for i, detection := range detections {
			// the position of the detection in the camera coordinate frame if it were at the movementsensor's location
			desiredPoint := r3.Vector{
				X: detection.Geometry.Pose().Point().X - cameraToMovementsensor.Pose().Point().X,
				Y: detection.Geometry.Pose().Point().Y - cameraToMovementsensor.Pose().Point().Y,
				Z: detection.Geometry.Pose().Point().Z - cameraToMovementsensor.Pose().Point().Z,
			}
			svc.logger.Infof("desiredPoint: %v", desiredPoint)

			desiredPose := spatialmath.NewPose(
				desiredPoint,
				detection.Geometry.Pose().Orientation(),
			)
			svc.logger.Infof("desiredPose: %v", spatialmath.PoseToProtobuf(desiredPose))

			transformBy := spatialmath.PoseBetweenInverse(detection.Geometry.Pose(), desiredPose)

			// get the manipulated geometry
			manipulatedGeom := detection.Geometry.Transform(transformBy)
			svc.logger.Infof("1 manipulatedGeom Pose: %v", spatialmath.PoseToProtobuf(manipulatedGeom.Pose()))

			// fix axes of geometry's pose such that it is in the cooordinate system of the base
			manipulatedGeom = manipulatedGeom.Transform(spatialmath.NewPoseFromOrientation(baseToCamera.Pose().Orientation()))
			svc.logger.Infof("2 manipulatedGeom Pose: %v", spatialmath.PoseToProtobuf(manipulatedGeom.Pose()))

			// get the geometry's lat & lng along with its heading with respect to north as a left handed value
			obstacleGeoPose := spatialmath.PoseToGeoPose(robotGeoPose, manipulatedGeom.Pose())
			svc.logger.Infof("obstacleGeoPose.Location(): %v", obstacleGeoPose.Location())
			svc.logger.Infof("obstacleGeoPose.Heading(): %v", obstacleGeoPose.Heading())

			// prefix the label of the geometry so we know it is transient and add extra info
			label := "transient_" + strconv.Itoa(i) + "_" + detector.CameraName.Name
			if detection.Geometry.Label() != "" {
				label += "_" + detection.Geometry.Label()
			}
			detection.Geometry.SetLabel(label)

			// determine the desired geometry pose
			desiredPose = spatialmath.NewPose(r3.Vector{X: 0, Y: 0, Z: 0}, detection.Geometry.Pose().Orientation())

			// calculate what we need to transform by
			transformBy = spatialmath.PoseBetweenInverse(detection.Geometry.Pose(), desiredPose)

			// set the geometry's pose to desiredPose
			manipulatedGeom = detection.Geometry.Transform(transformBy)

			// create the geo obstacle
			obstacle := spatialmath.NewGeoObstacle(obstacleGeoPose.Location(), []spatialmath.Geometry{manipulatedGeom})

			// add manipulatedGeom to list of geoObstacles we return
			geoObstacles = append(geoObstacles, obstacle)
		}
		fmt.Println(" ")
	}
	fmt.Println(" ")
	fmt.Println(" ")

	return geoObstacles, nil
}

func (svc *builtIn) getTransforms(ctx context.Context, cameraName string) (*referenceframe.PoseInFrame, *referenceframe.PoseInFrame, *referenceframe.PoseInFrame, error) {
	fs, err := svc.fsService.FrameSystem(ctx, nil)
	if err != nil {
		svc.logger.Infof("ERR: %v", err.Error())
		return nil, nil, nil, err
	}
	svc.logger.Infof("fs.FrameNames(): %v", fs.FrameNames())
	svc.logger.Debug("camera to ms")
	// determine transform from camera to movement sensor
	movementsensorOrigin := referenceframe.NewPoseInFrame(svc.movementSensor.Name().ShortName(), spatialmath.NewZeroPose())
	cameraToMovementsensor, err := svc.fsService.TransformPose(ctx, movementsensorOrigin, cameraName, nil)
	if err != nil {
		svc.logger.Infof("ERR: %v", err.Error())
		// here we make the assumption the movement sensor is coincident with the camera
		cameraToMovementsensor = movementsensorOrigin
	}

	svc.logger.Debug("base to ms")
	// determine transform from base to movement sensor
	baseToMovementSensor, err := svc.fsService.TransformPose(ctx, movementsensorOrigin, svc.base.Name().ShortName(), nil)
	if err != nil {
		svc.logger.Infof("ERR: %v", err.Error())
		// here we make the assumption the movement sensor is coincident with the base
		baseToMovementSensor = movementsensorOrigin
	}

	svc.logger.Debug("base to camera")
	// determine transform from base to camera
	cameraOrigin := referenceframe.NewPoseInFrame(cameraName, spatialmath.NewZeroPose())
	baseToCamera, err := svc.fsService.TransformPose(ctx, cameraOrigin, svc.base.Name().ShortName(), nil)
	if err != nil {
		svc.logger.Infof("ERR: %v", err.Error())
		// here we make the assumption the base is coincident with the camera
		baseToCamera = cameraOrigin
	}

	return cameraToMovementsensor, baseToMovementSensor, baseToCamera, nil
}

func (svc *builtIn) Paths(ctx context.Context, extra map[string]interface{}) ([]*navigation.Path, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return nil, errUnimplemented
}
