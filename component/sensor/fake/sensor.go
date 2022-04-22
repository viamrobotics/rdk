// Package fake implements a fake Sensor.
package fake

import (
	"context"
	"sync"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
)

func init() {
	registry.RegisterComponent(
		sensor.Subtype,
		"fake",
		registry.Component{Constructor: func(
			ctx context.Context,
			r robot.Robot,
			config config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return newSensor(config.Name), nil
		}})
}

func newSensor(name string) sensor.MinimalSensor {
	return &Sensor{Name: name}
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	mu   sync.Mutex
	Name string
}

// GetReadings always returns the set values.
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []interface{}{1, 2, 3}, nil
}

// Do echos back whatever was sent to it.
func (s *Sensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
