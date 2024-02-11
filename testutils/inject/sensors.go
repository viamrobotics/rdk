package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
)

// SensorsService represents a fake instance of a sensors service.
type SensorsService struct {
	sensors.Service
	name          resource.Name
	SensorsFunc   func(ctx context.Context, extra map[string]any) ([]resource.Name, error)
	ReadingsFunc  func(ctx context.Context, resources []resource.Name, extra map[string]any) ([]sensors.Readings, error)
	DoCommandFunc func(ctx context.Context,
		cmd map[string]any) (map[string]any, error)
}

// NewSensorsService returns a new injected sensors service.
func NewSensorsService(name string) *SensorsService {
	return &SensorsService{name: sensors.Named(name)}
}

// Name returns the name of the resource.
func (s *SensorsService) Name() resource.Name {
	return s.name
}

// Sensors call the injected Sensors or the real one.
func (s *SensorsService) Sensors(ctx context.Context, extra map[string]any) ([]resource.Name, error) {
	if s.SensorsFunc == nil {
		return s.Service.Sensors(ctx, extra)
	}
	return s.SensorsFunc(ctx, extra)
}

// Readings call the injected Readings or the real one.
func (s *SensorsService) Readings(ctx context.Context, names []resource.Name, extra map[string]any) ([]sensors.Readings, error) {
	if s.ReadingsFunc == nil {
		return s.Service.Readings(ctx, names, extra)
	}
	return s.ReadingsFunc(ctx, names, extra)
}

// DoCommand calls the injected DoCommand or the real variant.
func (s *SensorsService) DoCommand(ctx context.Context,
	cmd map[string]any,
) (map[string]any, error) {
	if s.DoCommandFunc == nil {
		return s.Service.DoCommand(ctx, cmd)
	}
	return s.DoCommandFunc(ctx, cmd)
}
