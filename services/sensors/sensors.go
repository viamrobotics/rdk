// Package sensors implements a sensors service.
package sensors

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	pb "go.viam.com/api/service/sensors/v1"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/rlog"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&pb.SensorsService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSensorsServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SensorsService_ServiceDesc,
		RPCClient: func(ctx context.Context, conn rpc.ClientConn, name string, logger golog.Logger) interface{} {
			return NewClientFromConn(ctx, conn, name, logger)
		},
		Reconfigurable: WrapWithReconfigurable,
		MaxInstance:    resource.DefaultMaxInstance,
	})
	resource.AddDefaultService(Named(resource.DefaultServiceName))
}

// A Readings ties both the sensor name and its reading together.
type Readings struct {
	Name     resource.Name
	Readings map[string]interface{}
}

// A Service centralizes all sensors into one place.
type Service interface {
	Sensors(ctx context.Context) ([]resource.Name, error)
	Readings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error)
}

var (
	_ = Service(&reconfigurableSensors{})
	_ = resource.Reconfigurable(&reconfigurableSensors{})
	_ = goutils.ContextCloser(&reconfigurableSensors{})
)

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("sensors")

// Subtype is a constant that identifies the sensor service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named sensor's typed resource name.
// RSDK-347 Implements senors's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// NewUnimplementedInterfaceError is used when there is a failed interface check.
func NewUnimplementedInterfaceError(actual interface{}) error {
	return utils.NewUnimplementedInterfaceError((Service)(nil), actual)
}

// FromRobot is a helper for getting the named sensor service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(resource)
	}
	return svc, nil
}

// FindFirstName returns name of first sensors service found.
func FindFirstName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return ""
}

// FirstFromRobot returns the first sensor service in this robot.
func FirstFromRobot(r robot.Robot) (Service, error) {
	name := FindFirstName(r)
	return FromRobot(r, name)
}

type reconfigurableSensors struct {
	mu     sync.RWMutex
	actual Service
}

func (svc *reconfigurableSensors) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.(resource.Updateable).Update(ctx, resources)
}

func (svc *reconfigurableSensors) Sensors(ctx context.Context) ([]resource.Name, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Sensors(ctx)
}

func (svc *reconfigurableSensors) Readings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.Readings(ctx, sensorNames)
}

func (svc *reconfigurableSensors) Close(ctx context.Context) error {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return goutils.TryClose(ctx, svc.actual)
}

// Reconfigure replaces the old Sensors service with a new Sensors.
func (svc *reconfigurableSensors) Reconfigure(ctx context.Context, newSvc resource.Reconfigurable) error {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	rSvc, ok := newSvc.(*reconfigurableSensors)
	if !ok {
		return utils.NewUnexpectedTypeError(svc, newSvc)
	}
	if err := goutils.TryClose(ctx, svc.actual); err != nil {
		rlog.Logger.Errorw("error closing old", "error", err)
	}
	svc.actual = rSvc.actual
	return nil
}

// WrapWithReconfigurable wraps a Sensors service as a Reconfigurable.
func WrapWithReconfigurable(s interface{}) (resource.Reconfigurable, error) {
	svc, ok := s.(Service)
	if !ok {
		return nil, NewUnimplementedInterfaceError(s)
	}

	if reconfigurable, ok := s.(*reconfigurableSensors); ok {
		return reconfigurable, nil
	}

	return &reconfigurableSensors{actual: svc}, nil
}
