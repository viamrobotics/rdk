package inject

import (
	"context"

	"go.viam.com/rdk/components/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	ReadingsFunc func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
}

// Readings calls the injected Readings or the real version.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if s.ReadingsFunc == nil {
		return s.Sensor.Readings(ctx, extra)
	}
	return s.ReadingsFunc(ctx, extra)
}

// DoCommand calls the injected DoCommand or the real version.
func (s *Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Sensor.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}
