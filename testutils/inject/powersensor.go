package inject

import (
	"context"

	"braces.dev/errtrace"
	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

// A PowerSensor reports information about voltage, current and power.
type PowerSensor struct {
	powersensor.PowerSensor
	name         resource.Name
	VoltageFunc  func(ctx context.Context, extra map[string]interface{}) (float64, bool, error)
	CurrentFunc  func(ctx context.Context, extra map[string]interface{}) (float64, bool, error)
	PowerFunc    func(ctx context.Context, extra map[string]interface{}) (float64, error)
	ReadingsFunc func(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error)
	DoFunc       func(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error)
	StatusFunc   func(ctx context.Context) (map[string]interface{}, error)
	CloseFunc    func() error
}

// NewPowerSensor returns a new injected movement sensor.
func NewPowerSensor(name string) *PowerSensor {
	return &PowerSensor{name: powersensor.Named(name)}
}

// Name returns the name of the resource.
func (i *PowerSensor) Name() resource.Name {
	return i.name
}

// DoCommand calls the injected DoCommand or the real version.
func (i *PowerSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.DoFunc == nil {
		return errtrace.Wrap2(i.PowerSensor.DoCommand(ctx, cmd))
	}
	return errtrace.Wrap2(i.DoFunc(ctx, cmd))
}

// Voltage func or passthrough.
func (i *PowerSensor) Voltage(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	if i.VoltageFunc == nil {
		return errtrace.Wrap3(i.PowerSensor.Voltage(ctx, cmd))
	}
	return errtrace.Wrap3(i.VoltageFunc(ctx, cmd))
}

// Current func or passthrough.
func (i *PowerSensor) Current(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	if i.CurrentFunc == nil {
		return errtrace.Wrap3(i.PowerSensor.Current(ctx, cmd))
	}
	return errtrace.Wrap3(i.CurrentFunc(ctx, cmd))
}

// Power func or passthrough.
func (i *PowerSensor) Power(ctx context.Context, cmd map[string]interface{}) (float64, error) {
	if i.PowerFunc == nil {
		return errtrace.Wrap2(i.PowerSensor.Power(ctx, cmd))
	}
	return errtrace.Wrap2(i.PowerFunc(ctx, cmd))
}

// Readings func or passthrough.
func (i *PowerSensor) Readings(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.ReadingsFunc == nil {
		return errtrace.Wrap2(i.PowerSensor.Readings(ctx, cmd))
	}
	return errtrace.Wrap2(i.ReadingsFunc(ctx, cmd))
}

// Close calls the injected Close or the real version.
func (i *PowerSensor) Close(ctx context.Context) error {
	if i.CloseFunc == nil {
		if i.PowerSensor == nil {
			return nil
		}
		return errtrace.Wrap(i.PowerSensor.Close(ctx))
	}
	return errtrace.Wrap(i.CloseFunc())
}

// Status calls the injected Status or the real version.
func (i *PowerSensor) Status(ctx context.Context) (map[string]interface{}, error) {
	if i.StatusFunc != nil {
		return errtrace.Wrap2(i.StatusFunc(ctx))
	}
	if i.PowerSensor != nil {
		return errtrace.Wrap2(i.PowerSensor.Status(ctx))
	}
	return map[string]interface{}{}, nil
}
