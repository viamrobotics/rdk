// Package fake implements a fake Sensor.
package fake

import (
	"context"
	"sync"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func init() {
	resource.RegisterComponent(
		sensor.API,
		resource.DefaultModelFamily.WithModel("fake"),
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger logging.Logger,
		) (sensor.Sensor, error) {
			return newSensor(conf.ResourceName(), logger), nil
		}})
}

func newSensor(name resource.Name, logger logging.Logger) sensor.Sensor {
	return &Sensor{
		Named:  name.AsNamed(),
		logger: logger,
	}
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	mu sync.Mutex
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

// Readings always returns the set values.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{"a": 1, "b": 2, "c": 3}, nil
}
