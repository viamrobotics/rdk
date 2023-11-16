package inject

import (
	"context"

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
		return i.PowerSensor.DoCommand(ctx, cmd)
	}
	return i.DoFunc(ctx, cmd)
}

// Voltage func or passthrough.
func (i *PowerSensor) Voltage(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	if i.VoltageFunc == nil {
		return i.PowerSensor.Voltage(ctx, cmd)
	}
	return i.VoltageFunc(ctx, cmd)
}

// Current func or passthrough.
func (i *PowerSensor) Current(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	if i.CurrentFunc == nil {
		return i.PowerSensor.Current(ctx, cmd)
	}
	return i.CurrentFunc(ctx, cmd)
}

// Power func or passthrough.
func (i *PowerSensor) Power(ctx context.Context, cmd map[string]interface{}) (float64, error) {
	if i.PowerFunc == nil {
		return i.PowerSensor.Power(ctx, cmd)
	}
	return i.PowerFunc(ctx, cmd)
}

// Readings func or passthrough.
func (i *PowerSensor) Readings(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	if i.ReadingsFunc == nil {
		return i.PowerSensor.Readings(ctx, cmd)
	}
	return i.ReadingsFunc(ctx, cmd)
}
