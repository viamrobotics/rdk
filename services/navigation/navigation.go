package navigation

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/go-errors/errors"
	geo "github.com/kellydunn/golang-geo"
	"github.com/mitchellh/mapstructure"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"

	"go.viam.com/core/base"
	"go.viam.com/core/config"
	"go.viam.com/core/registry"
	"go.viam.com/core/robot"
	"go.viam.com/core/sensor/gps"
)

func init() {
	registry.RegisterService(Type, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	},
	)

	config.RegisterServiceAttributeMapConverter(Type, func(attributes config.AttributeMap) (interface{}, error) {
		var conf Config
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{TagName: "json", Result: &conf})
		if err != nil {
			return nil, err
		}
		if err := decoder.Decode(attributes); err != nil {
			return nil, err
		}
		return &conf, nil
	}, &Config{})
}

// Mode describes what mode to operate the service in.
type Mode uint8

// The set of known modes.
const (
	ModeManual = Mode(iota)
	ModeWaypoint
)

// A Service controls the navigation for a robot.
type Service interface {
	Mode(ctx context.Context) (Mode, error)
	SetMode(ctx context.Context, mode Mode) error
	Close() error

	Location(ctx context.Context) (*geo.Point, error)

	// Waypoint
	Waypoints(ctx context.Context) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point) error
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error
}

// Type is the type of service.
const Type = config.ServiceType("navigation")

// Config describes how to configure the service.
type Config struct {
	Store    StoreConfig `json:"store"`
	BaseName string      `json:"base"`
	GPSName  string      `json:"gps"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	if err := config.Store.Validate(fmt.Sprintf("%s.%s", path, "store")); err != nil {
		return err
	}
	if config.BaseName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	if config.GPSName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "gps")
	}
	return nil
}

// New returns a new navigation service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	svcConfig := config.ConvertedAttributes.(*Config)
	base1, ok := r.BaseByName(svcConfig.BaseName)
	if !ok {
		return nil, errors.Errorf("no base named %q", svcConfig.BaseName)
	}
	s, ok := r.SensorByName(svcConfig.GPSName)
	if !ok {
		return nil, errors.Errorf("no gps named %q", svcConfig.GPSName)
	}
	gpsDevice, ok := s.(gps.GPS)
	if !ok {
		return nil, errors.Errorf("%q is not a GPS device", svcConfig.GPSName)
	}

	var store navStore
	switch svcConfig.Store.Type {
	case storeTypeMemory:
		store = newMemoryNavigationStore()
	case storeTypeMongoDB:
		var err error
		store, err = newMongoDBNavigationStore(ctx, svcConfig.Store.Config)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.Errorf("unknown store type %q", svcConfig.Store.Type)
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	navSvc := &navService{
		r:          r,
		store:      store,
		base:       base1,
		gpsDevice:  gpsDevice,
		logger:     logger,
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
	}
	return navSvc, nil
}

type navService struct {
	mu    sync.RWMutex
	r     robot.Robot
	store navStore
	mode  Mode

	base      base.Base
	gpsDevice gps.GPS

	logger                  golog.Logger
	cancelCtx               context.Context
	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
}

func (svc *navService) Mode(ctx context.Context) (Mode, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.mode, nil
}

func (svc *navService) SetMode(ctx context.Context, mode Mode) error {
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

	svc.mode = ModeManual
	switch mode {
	case ModeWaypoint:
		if err := svc.startWaypoint(); err != nil {
			return err
		}
		svc.mode = mode
	}
	return nil
}

func (svc *navService) startWaypoint() error {
	svc.activeBackgroundWorkers.Add(1)
	utils.PanicCapturingGo(func() {
		defer svc.activeBackgroundWorkers.Done()

		var path = []*geo.Point{}
		for {
			if !utils.SelectContextOrWait(svc.cancelCtx, 500*time.Millisecond) {
				return
			}

			currentLoc, err := svc.gpsDevice.Location(svc.cancelCtx)
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
				if _, err := svc.base.Spin(ctx, -1*bearingDelta, 45, true); err != nil {
					return fmt.Errorf("error turning: %w", err)
				}

				distanceMillis := distanceToGoal * 1000 * 1000
				distanceMillis = math.Min(distanceMillis, 10*1000)

				if _, err := svc.base.MoveStraight(ctx, int(distanceMillis), 500, true); err != nil {
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

func (svc *navService) waypointDirectionAndDistanceToGo(ctx context.Context, currentLoc *geo.Point) (float64, float64, error) {
	wp, err := svc.nextWaypoint(ctx)
	if err != nil {
		return 0, 0, err
	}

	goal := wp.ToPoint()

	return fixAngle(currentLoc.BearingTo(goal)), currentLoc.GreatCircleDistance(goal), nil
}

func (svc *navService) Location(ctx context.Context) (*geo.Point, error) {
	if svc.gpsDevice == nil {
		return nil, errors.New("no way to get location")
	}
	return svc.gpsDevice.Location(svc.cancelCtx)
}

func (svc *navService) Waypoints(ctx context.Context) ([]Waypoint, error) {
	wps, err := svc.store.Waypoints(ctx)
	if err != nil {
		return nil, err
	}
	wpsCopy := make([]Waypoint, 0, len(wps))
	wpsCopy = append(wpsCopy, wps...)
	return wpsCopy, nil
}

func (svc *navService) AddWaypoint(ctx context.Context, point *geo.Point) error {
	_, err := svc.store.AddWaypoint(ctx, point)
	return err
}

func (svc *navService) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	return svc.store.RemoveWaypoint(ctx, id)
}

func (svc *navService) nextWaypoint(ctx context.Context) (Waypoint, error) {
	return svc.store.NextWaypoint(ctx)
}

func (svc *navService) waypointReached(ctx context.Context) error {
	wp, err := svc.nextWaypoint(ctx)
	if err != nil {
		return fmt.Errorf("can't mark waypoint reached: %w", err)
	}
	return svc.store.WaypointVisited(ctx, wp.ID)

}

func (svc *navService) Close() error {
	svc.cancelFunc()
	svc.activeBackgroundWorkers.Wait()
	return utils.TryClose(svc.store)
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
