// Package navigation contains a navigation service, along with a gRPC server and client
package navigation

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
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/base"
	"go.viam.com/rdk/component/gps"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/navigation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	rdkutils "go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.NavigationService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterNavigationServiceHandlerFromEndpoint,
			)
		},
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	},
	)
	cType := config.ServiceType(SubtypeName)
	config.RegisterServiceAttributeMapConverter(cType, func(attributes config.AttributeMap) (interface{}, error) {
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
	GetMode(ctx context.Context) (Mode, error)
	SetMode(ctx context.Context, mode Mode) error
	Close(ctx context.Context) error

	GetLocation(ctx context.Context) (*geo.Point, error)

	// Waypoint
	GetWaypoints(ctx context.Context) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point) error
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("navigation")

// Subtype is a constant that identifies the navigation service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the NavigationService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

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
	svcConfig, ok := config.ConvertedAttributes.(*Config)
	if !ok {
		return nil, rdkutils.NewUnexpectedTypeError(svcConfig, config.ConvertedAttributes)
	}
	base1, err := base.FromRobot(r, svcConfig.BaseName)
	if err != nil {
		return nil, err
	}
	gpsDevice, err := gps.FromRobot(r, svcConfig.GPSName)
	if err != nil {
		return nil, err
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

func (svc *navService) GetMode(ctx context.Context) (Mode, error) {
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
	if mode == ModeWaypoint {
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

		path := []*geo.Point{}
		for {
			if !utils.SelectContextOrWait(svc.cancelCtx, 500*time.Millisecond) {
				return
			}

			currentLoc, err := svc.gpsDevice.ReadLocation(svc.cancelCtx)
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
				if err := svc.base.Spin(ctx, -1*bearingDelta, 45); err != nil {
					return fmt.Errorf("error turning: %w", err)
				}

				distanceMm := distanceToGoal * 1000 * 1000
				distanceMm = math.Min(distanceMm, 10*1000)

				if err := svc.base.MoveStraight(ctx, int(distanceMm), 500); err != nil {
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

func (svc *navService) GetLocation(ctx context.Context) (*geo.Point, error) {
	if svc.gpsDevice == nil {
		return nil, errors.New("no way to get location")
	}
	return svc.gpsDevice.ReadLocation(ctx)
}

func (svc *navService) GetWaypoints(ctx context.Context) ([]Waypoint, error) {
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

func (svc *navService) Close(ctx context.Context) error {
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
