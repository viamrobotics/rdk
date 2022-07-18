// Package fake implements a fake Sensor.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/generic"
	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
)

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSensor(config.Name), nil
		}})
}

func newSensor(name string) sensor.Sensor {
	return &Sensor{Name: name}
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	mu   sync.Mutex
	Name string
	generic.Echo
}

// GetReadings always returns the set values.
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []interface{}{1, 2, 3}, nil
}
