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
			return &Sensor{Name: config.Name}, nil
		}})
}

// Sensor is a fake Sensor device that always returns the set location.
type Sensor struct {
	mu   sync.Mutex
	Name string
}

// Readings always returns the set values.
func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return []interface{}{1}, nil
}

// Desc returns that this is a Sensor.
func (s *Sensor) Desc() sensor.Description {
	return sensor.Description{sensor.Type(sensor.SubtypeName), ""}
}
