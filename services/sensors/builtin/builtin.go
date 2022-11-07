// Package builtin implements the default sensors service.
package builtin

import (
	"context"
	"sync"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/sensors"
)

func init() {
	registry.RegisterService(sensors.Subtype, resource.DefaultModelName, registry.Service{
		Constructor: func(ctx context.Context, r robot.Robot, c config.Service, logger golog.Logger) (interface{}, error) {
			return NewBuiltIn(ctx, r, c, logger)
		},
	})
	resource.AddDefaultService(sensors.Named(resource.DefaultModelName))
}

// NewBuiltIn returns a new default sensor service for the given robot.
func NewBuiltIn(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (sensors.Service, error) {
	s := &builtIn{
		sensors: map[resource.Name]sensor.Sensor{},
		logger:  logger,
	}
	return s, nil
}

type builtIn struct {
	mu      sync.RWMutex
	sensors map[resource.Name]sensor.Sensor
	logger  golog.Logger
}

// Sensors returns all sensors in the robot.
func (s *builtIn) Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]resource.Name, 0, len(s.sensors))
	for name := range s.sensors {
		names = append(names, name)
	}
	return names, nil
}

// Readings returns the readings of the resources specified.
func (s *builtIn) Readings(ctx context.Context, sensorNames []resource.Name, extra map[string]interface{}) ([]sensors.Readings, error) {
	s.mu.RLock()
	// make a copy of sensors and then unlock
	sensorsMap := make(map[resource.Name]sensor.Sensor, len(s.sensors))
	for name, sensor := range s.sensors {
		sensorsMap[name] = sensor
	}
	s.mu.RUnlock()

	// dedupe sensorNames
	deduped := make(map[resource.Name]struct{}, len(sensorNames))
	for _, val := range sensorNames {
		deduped[val] = struct{}{}
	}

	readings := make([]sensors.Readings, 0, len(deduped))
	for name := range deduped {
		sensor, ok := sensorsMap[name]
		if !ok {
			return nil, errors.Errorf("resource %q not a registered sensor", name)
		}
		reading, err := sensor.Readings(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get reading from %q", name)
		}
		readings = append(readings, sensors.Readings{Name: name, Readings: reading})
	}
	return readings, nil
}

// Update updates the sensors service when the robot has changed.
func (s *builtIn) Update(ctx context.Context, resources map[resource.Name]interface{}) error {
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
