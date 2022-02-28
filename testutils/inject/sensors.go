package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
)

// SensorsService represents a fake instance of a sensors service.
type SensorsService struct {
	sensors.Service

	GetSensorsFunc  func(ctx context.Context) ([]resource.Name, error)
	GetReadingsFunc func(ctx context.Context, resources []resource.Name) ([]sensors.Readings, error)
}

// GetSensors call the injected GetSensors or the real one.
func (s *SensorsService) GetSensors(ctx context.Context) ([]resource.Name, error) {
	if s.GetSensorsFunc == nil {
		return s.Service.GetSensors(ctx)
	}
	return s.GetSensorsFunc(ctx)
}

// GetReadings call the injected GetReadings or the real one.
func (s *SensorsService) GetReadings(ctx context.Context, names []resource.Name) ([]sensors.Readings, error) {
	if s.GetReadingsFunc == nil {
		return s.Service.GetReadings(ctx, names)
	}
	return s.GetReadingsFunc(ctx, names)
}
