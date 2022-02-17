// Package sensors implements a sensors service.
package sensors

import (
	"context"

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
	GetReadings(ctx context.Context, resources []resource.Name) ([]Reading, error)
	GetSensors(ctx context.Context) ([]resource.Name, error)
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
		return nil, utils.NewUnimplementedInterfaceError("sensor.Service", resource)
	}
	return svc, nil
}

// New returns a new sensor service for the given robot.
func New(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (Service, error) {
	// look for sensors here

	return &sensorService{
		sensors: map[resource.Name]sensor.Sensor{},
		logger:  logger,
	}, nil
}

type sensorService struct {
	// add lock
	sensors map[resource.Name]sensor.Sensor
	logger  golog.Logger
}

// GetReadings returns the readings of the resources specified.
func (s sensorService) GetReadings(ctx context.Context, names []resource.Name) ([]Reading, error) {
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

// GetSensors returns all sensors in the robot.
func (s sensorService) GetSensors(ctx context.Context) ([]resource.Name, error) {
	names := make([]resource.Name, len(s.sensors))
	for name := range s.sensors {
		names = append(names, name)
	}
	return names, nil
}

// add update function
