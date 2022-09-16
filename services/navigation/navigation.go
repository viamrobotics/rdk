// Package navigation contains a navigation service, along with a gRPC server and client
package navigation

import (
	"context"
	"fmt"
	"sync"

	"github.com/edaniels/golog"
	geo "github.com/kellydunn/golang-geo"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.viam.com/utils"
	"go.viam.com/utils/rpc"

	servicepb "go.viam.com/rdk/proto/api/service/navigation/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
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
		RPCServiceDesc: &servicepb.NavigationService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
	})
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

	GetLocation(ctx context.Context) (*geo.Point, error)

	// Waypoint
	GetWaypoints(ctx context.Context) ([]Waypoint, error)
	AddWaypoint(ctx context.Context, point *geo.Point) error
	RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error
}

var (
	_ = Service(&reconfigurableNavigation{})
	_ = resource.Reconfigurable(&reconfigurableNavigation{})
	_ = utils.ContextCloser(&reconfigurableNavigation{})
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("navigation")

// Subtype is a constant that identifies the navigation service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named navigation service's typed resource name.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// Config describes how to configure the service.
type Config struct {
	Store              StoreConfig `json:"store"`
	BaseName           string      `json:"base"`
	MovementSensorName string      `json:"movement_sensor"`

	DegPerSecDefault float64 `json:"deg_per_sec"`
	MMPerSecDefault  float64 `json:"mm_per_sec"`
}

// Validate ensures all parts of the config are valid.
func (config *Config) Validate(path string) error {
	if err := config.Store.Validate(fmt.Sprintf("%s.%s", path, "store")); err != nil {
		return err
	}
	if config.BaseName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "base")
	}
	if config.MovementSensorName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "movement_sensor")
	}
	return nil
}

type reconfigurableNavigation struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableNavigation) GetMode(ctx context.Context) (Mode, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetMode(ctx)
}

func (svc *reconfigurableNavigation) SetMode(ctx context.Context, mode Mode) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.SetMode(ctx, mode)
}

func (svc *reconfigurableNavigation) GetLocation(ctx context.Context) (*geo.Point, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetLocation(ctx)
}

// Waypoint.
func (svc *reconfigurableNavigation) GetWaypoints(ctx context.Context) ([]Waypoint, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetWaypoints(ctx)
}

func (svc *reconfigurableNavigation) AddWaypoint(ctx context.Context, point *geo.Point) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.AddWaypoint(ctx, point)
}

func (svc *reconfigurableNavigation) RemoveWaypoint(ctx context.Context, id primitive.ObjectID) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.RemoveWaypoint(ctx, id)
}

func (svc *reconfigurableNavigation) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return utils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old navigation service with a new navigation.
func (svc *reconfigurableNavigation) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableNavigation)
	if !ok {
		return rdkutils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := utils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return rdkutils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

// WrapWithReconfigurable wraps a navigation service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableNavigation); ok {
		return reconfigurable, nil
	}

	return &reconfigurableNavigation{actual: svc}, nil
}
