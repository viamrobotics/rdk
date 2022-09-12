package inject

import (
	"context"

	"go.viam.com/rdk/components/sensor"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	DoFunc          func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	GetReadingsFunc func(ctx context.Context) (map[string]interface{}, error)
}

// GetReadings calls the injected GetReadings or the real version.
func (s *Sensor) GetReadings(ctx context.Context) (map[string]interface{}, error) {
	if s.GetReadingsFunc == nil {
		return s.Sensor.GetReadings(ctx)
	}
	return s.GetReadingsFunc(ctx)
}

// DoCommand calls the injected DoCommand or the real version.
func (s *Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return s.Sensor.DoCommand(ctx, cmd)
	}
	return s.DoFunc(ctx, cmd)
}
