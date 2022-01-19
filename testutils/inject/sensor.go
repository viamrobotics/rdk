package inject

import (
	"context"

	"go.viam.com/rdk/component/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	ReadingsFunc func(ctx context.Context) ([]interface{}, error)
	DescFunc     func(ctx context.Context) (sensor.Description, error)
}

// Readings calls the injected Readings or the real version.
func (s *Sensor) Readings(ctx context.Context) ([]interface{}, error) {
	if s.ReadingsFunc == nil {
		return s.Sensor.Readings(ctx)
	}
	return s.ReadingsFunc(ctx)
}

// Desc returns the description if available.
func (s *Sensor) Desc(ctx context.Context) (sensor.Description, error) {
	if s.DescFunc == nil {
		return s.Sensor.Desc(ctx)
	}
	return sensor.Description{Type: sensor.Type("sensor")}, nil
}
