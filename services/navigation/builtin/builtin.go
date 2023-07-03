// Package builtin contains the default navigation service, along with a gRPC server and client
package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
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
	metersPerSecDefault = 0.5
	degPerSecDefault    = 45
)

func init() {
	resource.RegisterService(navigation.API, resource.DefaultServiceModel, resource.Registration[navigation.Service, *Config]{
		Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (navigation.Service, error) {
			return NewBuiltIn(ctx, deps, conf, logger)
		},
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
		conf.MetersPerSec = metersPerSecDefault
	}
	if conf.DegPerSec == 0 {
		conf.DegPerSec = degPerSecDefault
	}

	return deps, nil
}

// NewBuiltIn returns a new navigation service for the given robot.
func NewBuiltIn(ctx context.Context, deps resource.Dependencies, conf resource.Config, logger golog.Logger) (navigation.Service, error) {
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	navSvc := &builtIn{
		Named:      conf.ResourceName().AsNamed(),
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	if err := navSvc.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return navSvc, nil
}

type builtIn struct {
	resource.Named
	mu        sync.RWMutex
	store     navigation.NavStore
	storeType string
	mode      navigation.Mode

	base           base.Base
	movementSensor movementsensor.MovementSensor
	motion         motion.Service
	obstacles      []*spatialmath.GeoObstacle

	metersPerSec            float64
	degPerSec               float64
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *builtIn) Reconfigure(ctx context.Context, deps resource.Dependencies, conf resource.Config) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()

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
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.mode == mode {
		return nil
	}

	// switch modes
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	svc.cancelCtx = cancelCtx
	svc.cancelFunc = cancelFunc
	svc.mode = navigation.ModeManual
	if mode == navigation.ModeWaypoint {
		if extra != nil && extra["experimental"] == true {
			if err := svc.startWaypointExperimental(extra); err != nil {
				return err
			}
		} else if err := svc.startWaypoint(extra); err != nil {
			return err
		}

		svc.mode = mode
	}
	return nil
}

func (svc *builtIn) computeCurrentBearing(ctx context.Context, path []*geo.Point) (float64, error) {
	props, err := svc.movementSensor.Properties(ctx, nil)
	if err != nil {
		return 0, err
	}
	if props.CompassHeadingSupported {
		return svc.movementSensor.CompassHeading(ctx, nil)
	}
	pathLen := len(path)
	return fixAngle(path[pathLen-2].BearingTo(path[pathLen-1])), nil
}

func (svc *builtIn) startWaypoint(extra map[string]interface{}) error {
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		path := []*geo.Point{}
		for {
			if !utils.SelectContextOrWait(svc.cancelCtx, 500*time.Millisecond) {
				return
			}
			currentLoc, _, err := svc.movementSensor.Position(svc.cancelCtx, extra)
			if err != nil {
				svc.logger.Errorw("failed to get gps location", "error", err)
				continue
			}

			if len(path) <= 1 || currentLoc.GreatCircleDistance(path[len(path)-1]) > .0001 {
				// gps often updates less frequently
				path = append(path, currentLoc)
				if len(path) > 2 {
					path = path[len(path)-2:]
				}
			}

			navOnce := func(ctx context.Context) error {
				if len(path) <= 1 {
					return errors.New("not enough gps data")
				}

				currentBearing, err := svc.computeCurrentBearing(ctx, path)
				if err != nil {
					return err
				}

				bearingToGoal, distanceToGoal, err := svc.waypointDirectionAndDistanceToGo(ctx, currentLoc)
				if err != nil {
					return err
				}

				if distanceToGoal < .005 {
					svc.logger.Debug("i made it")
					return svc.waypointReached(ctx)
				}

				bearingDelta := computeBearing(bearingToGoal, currentBearing)
				steeringDir := -bearingDelta / 180.0

				svc.logger.Debugf("currentBearing: %0.0f bearingToGoal: %0.0f distanceToGoal: %0.3f bearingDelta: %0.1f steeringDir: %0.2f",
					currentBearing, bearingToGoal, distanceToGoal, bearingDelta, steeringDir)

				// TODO(erh->erd): maybe need an arc/stroke abstraction?
				// - Remember that we added -1*bearingDelta instead of steeringDir
				// - Test both naval/land to prove it works
				if err := svc.base.Spin(ctx, -1*bearingDelta, svc.degPerSec, nil); err != nil {
					return fmt.Errorf("error turning: %w", err)
				}

				distanceMm := distanceToGoal * 1000 * 1000
				distanceMm = math.Min(distanceMm, 10*1000)

				// TODO: handle swap from mm to meters
				if err := svc.base.MoveStraight(ctx, int(distanceMm), (svc.metersPerSec * 1000), nil); err != nil {
					return fmt.Errorf("error moving %w", err)
				}

				return nil
			}

			if err := navOnce(svc.cancelCtx); err != nil {
				svc.logger.Infof("error navigating: %s", err)
			}
		}
	})
	return nil
}

func (svc *builtIn) waypointDirectionAndDistanceToGo(ctx context.Context, currentLoc *geo.Point) (float64, float64, error) {
	wp, err := svc.nextWaypoint(ctx)
	if err != nil {
		return 0, 0, err
	}

	goal := wp.ToPoint()

	return fixAngle(currentLoc.BearingTo(goal)), currentLoc.GreatCircleDistance(goal), nil
}

func (svc *builtIn) Location(ctx context.Context, extra map[string]interface{}) (*geo.Point, error) {
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
	return svc.store.RemoveWaypoint(ctx, id)
}

func (svc *builtIn) nextWaypoint(ctx context.Context) (navigation.Waypoint, error) {
	return svc.store.NextWaypoint(ctx)
}

func (svc *builtIn) waypointReached(ctx context.Context) error {
	wp, err := svc.nextWaypoint(ctx)
	if err != nil {
		return fmt.Errorf("can't mark waypoint reached: %w", err)
	}
	return svc.store.WaypointVisited(ctx, wp.ID)
}

func (svc *builtIn) Close(ctx context.Context) error {
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	return svc.store.Close(ctx)
}

func fixAngle(a float64) float64 {
	for a < 0 {
		a += 360
	}
	for a > 360 {
		a -= 360
	}
	return a
}

func computeBearing(a, b float64) float64 {
	a = fixAngle(a)
	b = fixAngle(b)

	t := b - a
	if t < -180 {
		t += 360
	}

	if t > 180 {
		t -= 360
	}

	return t
}

func (svc *builtIn) startWaypointExperimental(extra map[string]interface{}) error {
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		navOnce := func(ctx context.Context, wp navigation.Waypoint) error {
			if extra == nil {
				extra = map[string]interface{}{"motion_profile": "position_only"}
			} else if extra["motion_profile"] == nil {
				extra["motion_profile"] = "position_only"
			}

			goal := wp.ToPoint()
			_, err := svc.motion.MoveOnGlobe(
				ctx,
				svc.base.Name(),
				goal,
				math.NaN(),
				svc.movementSensor.Name(),
				svc.obstacles,
				svc.metersPerSec*1000,
				svc.degPerSec,
				extra,
			)
			if err != nil {
				return err
			}

			return svc.waypointReached(ctx)
		}

		// loop until no waypoints remaining
		for wp, err := svc.nextWaypoint(svc.cancelCtx); err == nil; wp, err = svc.nextWaypoint(svc.cancelCtx) {
			svc.logger.Infof("navigating to waypoint: %+v", wp)
			if err := navOnce(svc.cancelCtx, wp); err != nil {
				svc.logger.Infof("skipping waypoint %+v due to error while navigating towards it: %s", wp, err)
			}
		}

	})
	return nil
}
