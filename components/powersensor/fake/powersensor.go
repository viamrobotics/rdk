// Package fake is a fake PowerSensor for testing
package fake

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

const defaultBatteryPercent = 76.5

// Config is used for converting fake powersensor attributes.
type Config struct {
	BatteryPercent *float64 `json:"battery_percent,omitempty"`
	// IsCharging also controls IsChargingSupported: when unset, the fake reports no charging support.
	IsCharging *bool `json:"is_charging,omitempty"`
}

// Validate ensures battery_percent is a percentage when set.
func (conf *Config) Validate(path string) ([]string, []string, error) {
	if conf.BatteryPercent != nil && (*conf.BatteryPercent < 0 || *conf.BatteryPercent > 100) {
		return nil, nil, resource.NewConfigValidationError(path,
			fmt.Errorf("battery_percent must be between 0 and 100, got %v", *conf.BatteryPercent))
	}
	return nil, nil, nil
}

func init() {
	resource.RegisterComponent(
		powersensor.API,
		model,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: newFakePowerSensorModel,
		},
	)
}

func newFakePowerSensorModel(_ context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
) (powersensor.PowerSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}
	batteryPercent := defaultBatteryPercent
	if newConf.BatteryPercent != nil {
		batteryPercent = *newConf.BatteryPercent
	}
	return powersensor.PowerSensor(&PowerSensor{
		Named:          conf.ResourceName().AsNamed(),
		logger:         logger,
		batteryPercent: batteryPercent,
		isCharging:     newConf.IsCharging,
	}), nil
}

// PowerSensor implements a fake PowerSensor interface.
type PowerSensor struct {
	resource.Named
	resource.AlwaysRebuild
	logger         logging.Logger
	batteryPercent float64
	isCharging     *bool
}

// DoCommand uses a map string to run custom functionality of a fake powersensor.
func (f *PowerSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// Voltage gets the voltage and isAC of a fake powersensor.
func (f *PowerSensor) Voltage(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 1.5, true, nil
}

// Current gets the current and isAC of a fake powersensor.
func (f *PowerSensor) Current(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 2.2, true, nil
}

// Power gets the power of a fake powersensor.
func (f *PowerSensor) Power(ctx context.Context, cmd map[string]interface{}) (float64, error) {
	return 9.8, nil
}

// BatteryPercent gets the battery charge percentage and charging state of a fake powersensor.
func (f *PowerSensor) BatteryPercent(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return f.batteryPercent, f.isCharging != nil && *f.isCharging, nil
}

// Properties returns the properties of a fake powersensor.
func (f *PowerSensor) Properties(ctx context.Context, cmd map[string]interface{}) (*powersensor.Properties, error) {
	return &powersensor.Properties{IsBatteryPercentSupported: true, IsChargingSupported: f.isCharging != nil}, nil
}

// Readings gets the readings of a fake powersensor.
func (f *PowerSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	volts, isAC, err := f.Voltage(ctx, nil)
	if err != nil {
		f.logger.CErrorf(ctx, "failed to get voltage reading: %s", err.Error())
	}

	amps, _, err := f.Current(ctx, nil)
	if err != nil {
		f.logger.CErrorf(ctx, "failed to get current reading: %s", err.Error())
	}

	watts, err := f.Power(ctx, nil)
	if err != nil {
		f.logger.CErrorf(ctx, "failed to get power reading: %s", err.Error())
	}
	return map[string]interface{}{
		"volts": volts,
		"amps":  amps,
		"is_ac": isAC,
		"watts": watts,
	}, nil
}

// Close closes the fake powersensor.
func (f *PowerSensor) Close(ctx context.Context) error {
	return nil
}
