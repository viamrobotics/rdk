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

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/navigation"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	defaultMetersPerSec = 0.5
	defaultDegPerSec    = 45
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

// Config describes how to configure the service.
type Config struct {
	Store              navigation.StoreConfig `json:"store"`
	BaseName           string                 `json:"base"`
	MovementSensorName string                 `json:"movement_sensor"`
	MotionServiceName  string                 `json:"motion_service"`
	// DegPerSec and MetersPerSec are targets and not hard limits on speed
	DegPerSec    float64                          `json:"degs_per_sec"`
	MetersPerSec float64                          `json:"meters_per_sec"`
	Obstacles    []*spatialmath.GeoObstacleConfig `json:"obstacles,omitempty"`
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

	// get default speeds from config if set, else defaults from nav services const
	if conf.MetersPerSec == 0 {
		conf.MetersPerSec = defaultMetersPerSec
	}
	if conf.DegPerSec == 0 {
		conf.DegPerSec = defaultDegPerSec
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

type builtIn struct {
	resource.Named
	actionMu  sync.RWMutex
	mu        sync.RWMutex
	store     navigation.NavStore
	storeType string
	mode      navigation.Mode

	base           base.Base
	movementSensor movementsensor.MovementSensor
	motion         motion.Service
	obstacles      []*spatialmath.GeoObstacle

	metersPerSec              float64
	degPerSec                 float64
	logger                    golog.Logger
	wholeServiceCancelFunc    func()
	currentWaypointCancelFunc func()
	waypointInProgress        *navigation.Waypoint
	activeBackgroundWorkers   sync.WaitGroup
}

func (svc *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.actionMu.Lock()
	defer svc.actionMu.Unlock()

	if svc.wholeServiceCancelFunc != nil {
		svc.wholeServiceCancelFunc()
	}
	svc.activeBackgroundWorkers.Wait()

	svcConfig, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}
	base1, err := base.FromDependencies(deps, svcConfig.BaseName)
	if err != nil {
		return err
	}
	movementSensor, err := movementsensor.FromDependencies(deps, svcConfig.MovementSensorName)
	if err != nil {
		return err
	}
	motionSrv, err := motion.FromDependencies(deps, svcConfig.MotionServiceName)
	if err != nil {
		return err
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
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
	svc.base = base1
	svc.movementSensor = movementSensor
	svc.motion = motionSrv
	svc.obstacles = newObstacles
	svc.metersPerSec = svcConfig.MetersPerSec
	svc.degPerSec = svcConfig.DegPerSec

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
	if svc.mode == mode {
		svc.mu.RUnlock()
		return nil
	}
	svc.mu.RUnlock()

	// switch modes
	if svc.wholeServiceCancelFunc != nil {
		svc.wholeServiceCancelFunc()
	}
	svc.activeBackgroundWorkers.Wait()

	svc.mu.Lock()
	defer svc.mu.Unlock()
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	svc.wholeServiceCancelFunc = cancelFunc
	svc.mode = mode
	if svc.mode == navigation.ModeWaypoint {
		svc.startWaypoint(cancelCtx, extra)
	}
	return nil
}

func (svc *builtIn) Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

	if svc.movementSensor == nil {
		return nil, errors.New("no way to get location")
	}
	loc, _, err := svc.movementSensor.Position(ctx, extra)
	return loc, err
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
	_, err := svc.store.AddWaypoint(ctx, point)
	return err
}

func (svc *builtIn) RemoveWaypoint(ctx context.Context, id primitive.ObjectID, extra map[string]interface{}) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
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

	if svc.wholeServiceCancelFunc != nil {
		svc.wholeServiceCancelFunc()
	}
	svc.activeBackgroundWorkers.Wait()

	return svc.store.Close(ctx)
}

func (svc *builtIn) startWaypoint(ctx context.Context, extra map[string]interface{}) {
	if extra == nil {
		extra = map[string]interface{}{"motion_profile": "position_only"}
	} else if _, ok := extra["motion_profile"]; !ok {
		extra["motion_profile"] = "position_only"
	}

	motionCfg := motion.MotionConfiguration{
		LinearMPerSec:     svc.metersPerSec,
		AngularDegsPerSec: svc.degPerSec,
	}

	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		navOnce := func(ctx context.Context, wp navigation.Waypoint) error {
			_, err := svc.motion.MoveOnGlobe(
				ctx,
				svc.base.Name(),
				wp.ToPoint(),
				math.NaN(),
				svc.movementSensor.Name(),
				svc.obstacles,
				&motionCfg,
				extra,
			)
			if err != nil {
				return err
			}

			return svc.waypointReached(ctx)
		}

		// loop until no waypoints remaining
		for {
			if ctx.Err() != nil {
				return
			}

			wp, err := svc.store.NextWaypoint(ctx)
			if err != nil {
				return
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
					svc.logger.Info("can't mark waypoint %+v as reached, exiting navigation due to error: %s", wp, err)
					return
				}
			}
		}
	})
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
