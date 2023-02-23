package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
)

// SensorsService represents a fake instance of a sensors service.
type SensorsService struct {
	sensors.Service

	SensorsFunc   func(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error)
	ReadingsFunc  func(ctx context.Context, resources []resource.Name, extra map[string]interface{}) ([]sensors.Readings, error)
	DoCommandFunc func(ctx context.Context,
		cmd map[string]interface{}) (map[string]interface{}, error)
}

// Sensors call the injected Sensors or the real one.
func (s *SensorsService) Sensors(ctx context.Context, extra map[string]interface{}) ([]resource.Name, error) {
	if s.SensorsFunc == nil {
		return s.Service.Sensors(ctx, extra)
	}
	return s.SensorsFunc(ctx, extra)
}

// Readings call the injected Readings or the real one.
func (s *SensorsService) Readings(ctx context.Context, names []resource.Name, extra map[string]interface{}) ([]sensors.Readings, error) {
	if s.ReadingsFunc == nil {
		return s.Service.Readings(ctx, names, extra)
	}
	return s.ReadingsFunc(ctx, names, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (s *SensorsService) DoCommand(ctx context.Context,
	cmd map[string]interface{},
) (map[string]interface{}, error) {
	if s.DoCommandFunc == nil {
		return s.Service.DoCommand(ctx, cmd)
	}
	return s.DoCommandFunc(ctx, cmd)
}
