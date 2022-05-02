package inject

import (
	"context"

	"go.viam.com/rdk/component/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	DoFunc          func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetReadingsFunc func(ctx context.Context) ([]interface{}, error)
}

// GetReadings calls the injected GetReadings or the real version.
func (s *Sensor) GetReadings(ctx context.Context) ([]interface{}, error) {
	if s.GetReadingsFunc == nil {
		return s.Sensor.GetReadings(ctx)
	}
	return s.GetReadingsFunc(ctx)
}

// Do calls the injected Do or the real version.
func (s *Sensor) Do(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Sensor.Do(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}
