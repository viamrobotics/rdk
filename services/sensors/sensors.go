// Package sensors implements a sensors service.
package sensors

import (
	"context"
	"strings"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils/rpc"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	pb "go.viam.com/rdk/proto/api/service/sensors/v1"
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
				&pb.SensorsService_ServiceDesc,
				NewServer(subtypeSvc),
				pb.RegisterSensorsServiceHandlerFromEndpoint,
			)
		},
		RPCServiceDesc: &pb.SensorsService_ServiceDesc,
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

// Named is a helper for getting the named sensor's typed resource name.
func Named(name string) resource.Name {
	remotes := strings.Split(name, ":")
	if len(remotes) > 1 {
		rName := resource.NameFromSubtype(Subtype, "")
		rName.PrependRemote(resource.RemoteName(strings.Join(remotes[:len(remotes)-1], ":")))
		return rName
	}
	return resource.NameFromSubtype(Subtype, "")
}

// FromRobot retrieves the sensor service of a robot.
func FromRobot(r robot.Robot) (Service, error) {
	resource, err := r.ResourceByName(Name)
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
