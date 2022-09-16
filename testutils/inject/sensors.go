package inject

import (
	"context"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/sensors"
)

// SensorsService represents a fake instance of a sensors service.
type SensorsService struct {
	sensors.Service

	GetSensorsFunc func(ctx context.Context) ([]resource.Name, error)
	ReadingsFunc   func(ctx context.Context, resources []resource.Name) ([]sensors.Readings, error)
}

// Sensors call the injected Sensors or the real one.
func (s *SensorsService) Sensors(ctx context.Context) ([]resource.Name, error) {
	if s.GetSensorsFunc == nil {
		return s.Service.Sensors(ctx)
	}
	return s.GetSensorsFunc(ctx)
}

// Readings call the injected Readings or the real one.
func (s *SensorsService) Readings(ctx context.Context, names []resource.Name) ([]sensors.Readings, error) {
	if s.ReadingsFunc == nil {
		return s.Service.Readings(ctx, names)
	}
	return s.ReadingsFunc(ctx, names)
}
