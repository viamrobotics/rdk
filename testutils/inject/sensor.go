package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/resource"
)

// Sensor is an injected sensor.
type Sensor struct {
	sensor.Sensor
	name         resource.Name
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc   func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc    func(ctx context.Context) error
	ReadingsFunc func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
}

// NewSensor returns a new injected sensor.
func NewSensor(name string) *Sensor {
	return &Sensor{name: sensor.Named(name)}
}

// Name returns the name of the resource.
func (s *Sensor) Name() resource.Name {
	return s.name
}

// Close calls the injected Close or the real version.
func (s *Sensor) Close(ctx context.Context) error {
	if s.CloseFunc == nil {
		if s.Sensor == nil {
			return nil
		}
		return errtrace.Wrap(s.Sensor.Close(ctx))
	}
	return errtrace.Wrap(s.CloseFunc(ctx))
}

// Readings calls the injected Readings or the real version.
func (s *Sensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	if s.ReadingsFunc == nil {
		return errtrace.Wrap2(s.Sensor.Readings(ctx, extra))
	}
	return errtrace.Wrap2(s.ReadingsFunc(ctx, extra))
}

// DoCommand calls the injected DoCommand or the real version.
func (s *Sensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if s.DoFunc == nil {
		return errtrace.Wrap2(s.Sensor.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(s.DoFunc(ctx, cmd))
}

// Status calls the injected Status or the real version.
func (s *Sensor) Status(ctx context.Context) (map[string]interface{}, error) {
	if s.StatusFunc != nil {
		return errtrace.Wrap2(s.StatusFunc(ctx))
	}
	if s.Sensor != nil {
		return errtrace.Wrap2(s.Sensor.Status(ctx))
	}
	return map[string]interface{}{}, nil
}
