package inject

import (
	"context"

	"go.viam.com/rdk/component/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	ReadingsFunc func(ctx context.Context) ([]interface{}, error)
}

// Readings calls the injected Readings or the real version.
func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
	if s.ReadingsFunc == nil {
		return s.Sensor.Readings(ctx)
	}
	return s.ReadingsFunc(ctx)
}
