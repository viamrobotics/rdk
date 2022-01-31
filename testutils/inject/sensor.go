package inject

import (
	"context"

	"go.viam.com/rdk/component/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	GetReadingsFunc func(ctx context.Context) ([]interface{}, error)
}

// GetReadings calls the injected GetReadings or the real version.
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	if s.GetReadingsFunc == nil {
		return s.Sensor.GetReadings(ctx)
	}
	return s.GetReadingsFunc(ctx)
}
