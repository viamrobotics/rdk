// Package sensors implements a sensors service.
package sensors

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	servicepb "go.viam.com/rdk/proto/api/service/v1"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/subtype"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterResourceSubtype(Subtype, registry.ResourceSubtype{
		RegisterRPCService: func(ctx context.Context, rpcServer rpc.Server, subtypeSvc subtype.Service) error {
			return rpcServer.RegisterServiceServer(
				ctx,
				&servicepb.SensorsService_ServiceDesc,
				NewServer(subtypeSvc),
				servicepb.RegisterSensorsServiceHandlerFromEndpoint,
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
	})
}

// A Reading ties both the sensor name and its reading together.
type Reading struct {
	Name    resource.Name
	Reading []interface{}
}

// A Service centralizes all sensors into one place.
type Service interface {
	GetSensors(ctx context.Context) ([]resource.Name, error)
	GetReadings(ctx context.Context, resources []resource.Name) ([]Reading, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("sensors")

// Subtype is a constant that identifies the sensor service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Name is the SensorService's typed resource name.
var Name = resource.NameFromSubtype(Subtype, "")

// FromRobot retrieves the sensor service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, ok := r.ResourceByName(Name)
	if !ok {
		return nil, utils.NewResourceNotFoundError(Name)
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

	// trigger an update here
	resources := map[resource.Name]interface{}{}
	for _, n := range r.ResourceNames() {
		res, ok := r.ResourceByName(n)
		if !ok {
			return nil, utils.NewResourceNotFoundError(n)
		}
		resources[n] = res
	}
	if err := s.Update(ctx, resources); err != nil {
		return nil, err
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
func (s *sensorsService) GetReadings(ctx context.Context, names []resource.Name) ([]Reading, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	readings := make([]Reading, 0, len(names))
	for _, name := range names {
		sensor, ok := s.sensors[name]
		if !ok {
			return nil, errors.Errorf("resource %q not a registered sensor", name)
		}
		reading, err := sensor.GetReadings(ctx)
		if err != nil {
			return nil, err
		}
		readings = append(readings, Reading{Name: name, Reading: reading})
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
