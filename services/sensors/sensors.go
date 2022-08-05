// Package sensors implements a sensors service.
package sensors

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/sensors/v1"
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
	})
	registry.RegisterService(Subtype, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return New(ctx, r, c, logger)
		},
	})

	resource.AddDefaultService(Name)
}

// A Readings ties both the sensor name and its reading together.
type Readings struct {
	Name     resource.Name
	Readings []interface{}
}

// A Service centralizes all sensors into one place.
type Service interface {
	GetSensors(ctx context.Context) ([]resource.Name, error)
	GetReadings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error)
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

// Returns name of first vision service found. There should only be one
func FindSensorsName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return ""
}

// FromRobot retrieves the sensor service of a robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	resource, err := r.ResourceByName(Named(name))
	if err != nil {
		return nil, err
	}
	svc, ok := resource.(Service)
	if !ok {
		return nil, utils.NewUnimplementedInterfaceError("sensors.Service", resource)
	}
	return svc, nil
}

// New returns a new sensor service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	s := &sensorsService{
		sensors: map[resource.Name]sensor.Sensor{},
		logger:  logger,
	}
	return s, nil
}

type sensorsService struct {
	mu      sync.RWMutex
	sensors map[resource.Name]sensor.Sensor
	logger  golog.Logger
}

// GetSensors returns all sensors in the robot.
func (s *sensorsService) GetSensors(ctx context.Context) ([]resource.Name, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]resource.Name, 0, len(s.sensors))
	for name := range s.sensors {
		names = append(names, name)
	}
	return names, nil
}

// GetReadings returns the readings of the resources specified.
func (s *sensorsService) GetReadings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error) {
	s.mu.RLock()
	// make a copy of sensors and then unlock
	sensors := make(map[resource.Name]sensor.Sensor, len(s.sensors))
	for name, sensor := range s.sensors {
		sensors[name] = sensor
	}
	s.mu.RUnlock()

	// dedupe sensorNames
	deduped := make(map[resource.Name]struct{}, len(sensorNames))
	for _, val := range sensorNames {
		deduped[val] = struct{}{}
	}

	readings := make([]Readings, 0, len(deduped))
	for name := range deduped {
		sensor, ok := sensors[name]
		if !ok {
			return nil, errors.Errorf("resource %q not a registered sensor", name)
		}
		reading, err := sensor.GetReadings(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get reading from %q", name)
		}
		readings = append(readings, Readings{Name: name, Readings: reading})
	}
	return readings, nil
}

// Update updates the sensors service when the robot has changed.
func (s *sensorsService) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sensors := map[resource.Name]sensor.Sensor{}
	for n, r := range resources {
		if sensor, ok := r.(sensor.Sensor); ok {
			sensors[n] = sensor
		}
	}
	s.sensors = sensors
	return nil
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

func (svc *reconfigurableSensors) GetSensors(ctx context.Context) ([]resource.Name, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetSensors(ctx)
}

func (svc *reconfigurableSensors) GetReadings(ctx context.Context, sensorNames []resource.Name) ([]Readings, error) {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.actual.GetReadings(ctx, sensorNames)
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
		return nil, utils.NewUnimplementedInterfaceError("sensors.Service", s)
	}

	if reconfigurable, ok := s.(*reconfigurableSensors); ok {
		return reconfigurable, nil
	}

	return &reconfigurableSensors{actual: svc}, nil
}
