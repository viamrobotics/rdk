// Package builtin contains the default navigation service, along with a gRPC server and client
package builtin

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/base"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/navigation"
	rdkutils "go.viam.com/rdk/utils"
)

const (
	mmPerSecDefault  = 500
	degPerSecDefault = 45
)

func init() {
	registry.RegisterService(navigation.Subtype, resource.DefaultServiceModel, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, deps, c, logger)
		},
	},
	)
	config.RegisterServiceAttributeMapConverter(navigation.Subtype, resource.DefaultServiceModel,
		func(attributes config.AttributeMap) (interface{}, error) {
			var conf Config
			decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
			if err != nil {
				return nil, err
			}
			if err := decoder.Decode(attributes); err != nil {
				return nil, err
			}
			return &conf, nil
		}, &Config{},
	)
}

// Config describes how to configure the service.
type Config struct {
	Store              navigation.StoreConfig `json:"store"`
	BaseName           string                 `json:"base"`
	MovementSensorName string                 `json:"movement_sensor"`
	DegPerSecDefault   float64                `json:"degs_per_sec"`
	MMPerSecDefault    float64                `json:"mm_per_sec"`
}

// Validate creates the list of implicit dependencies.
func (config *Config) Validate(path string) ([]string, error) {
	var deps []string

	if config.BaseName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	deps = append(deps, config.BaseName)

	if config.MovementSensorName == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}
	deps = append(deps, config.MovementSensorName)

	return deps, nil
}

// NewBuiltIn returns a new navigation service for the given robot.
func NewBuiltIn(ctx context.Context, deps registry.Dependencies, config config.Service, logger golog.Logger) (navigation.Service, error) {
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	base1, err := base.FromDependencies(deps, svcConfig.BaseName)
	if err != nil {
		return nil, err
	}
	movementSensor, err := movementsensor.FromDependencies(deps, svcConfig.MovementSensorName)
	if err != nil {
		return nil, err
	}

	var store navigation.NavStore
	switch svcConfig.Store.Type {
	case navigation.StoreTypeMemory:
		store = navigation.NewMemoryNavigationStore()
	case navigation.StoreTypeMongoDB:
		var err error
		store, err = navigation.NewMongoDBNavigationStore(ctx, svcConfig.Store.Config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("unknown store type %q", svcConfig.Store.Type)
	}

	// get default speeds from config if set, else defaults from nav services const
	straightSpeed := svcConfig.MMPerSecDefault
	if straightSpeed == 0 {
		straightSpeed = mmPerSecDefault
	}
	spinSpeed := svcConfig.DegPerSecDefault
	if spinSpeed == 0 {
		spinSpeed = degPerSecDefault
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	navSvc := &builtIn{
		store:            store,
		base:             base1,
		movementSensor:   movementSensor,
		mmPerSecDefault:  straightSpeed,
		degPerSecDefault: spinSpeed,
		logger:           logger,
		cancelCtx:        cancelCtx,
		cancelFunc:       cancelFunc,
	}
	return navSvc, nil
}

type builtIn struct {
	generic.Unimplemented
	mu    sync.RWMutex
	store navigation.NavStore
	mode  navigation.Mode

	base           base.Base
	movementSensor movementsensor.MovementSensor

	mmPerSecDefault         float64
	degPerSecDefault        float64
	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
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
		if err := svc.startWaypoint(extra); err != nil {
			return err
		}
		svc.mode = mode
	}
	return nil
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

				pathLen := len(path)
				currentBearing := fixAngle(path[pathLen-2].BearingTo(path[pathLen-1]))

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
				if err := svc.base.Spin(ctx, -1*bearingDelta, svc.degPerSecDefault, nil); err != nil {
					return fmt.Errorf("error turning: %w", err)
				}

				distanceMm := distanceToGoal * 1000 * 1000
				distanceMm = math.Min(distanceMm, 10*1000)

				if err := svc.base.MoveStraight(ctx, int(distanceMm), svc.mmPerSecDefault, nil); err != nil {
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
	return utils.TryClose(ctx, svc.store)
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
