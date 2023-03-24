// Package fake implements a fake Sensor.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
)

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		resource.NewDefaultModel("fake"),
		registry.Component{Constructor: func(
			ctx context.Context,
			deps resource.Dependencies,
			conf resource.Config,
			logger golog.Logger,
		) (resource.Resource, error) {
			return newSensor(conf.ResourceName()), nil
		}})
}

func newSensor(name resource.Name) sensor.Sensor {
	return &Sensor{
		Named: name.AsNamed(),
	}
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	mu sync.Mutex
	resource.Named
	resource.TriviallyReconfigurable
}

// Readings always returns the set values.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{"a": 1, "b": 2, "c": 3}, nil
}
