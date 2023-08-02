// Package fake is a fake PowerSensor for testing
package fake

import (
	"context"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/components/powersensor"
	"go.viam.com/rdk/resource"
)

var model = resource.DefaultModelFamily.WithModel("fake")

// Config is used for converting fake movementsensor attributes.
type Config struct {
	resource.TriviallyValidateConfig
	ConnectionType string `json:"connection_type,omitempty"`
}

func init() {
	resource.RegisterComponent(
		powersensor.API,
		model,
		resource.Registration[powersensor.PowerSensor, *Config]{
			Constructor: func(
				ctx context.Context,
				deps resource.Dependencies,
				conf resource.Config,
				logger golog.Logger,
			) (powersensor.PowerSensor, error) {
				return powersensor.PowerSensor(&PowerSensor{
					Named: conf.ResourceName().AsNamed(),
				}), nil
			},
		})
}

// PowerSensor implements a fake PowerSensor interface.
type PowerSensor struct {
	resource.Named
	resource.AlwaysRebuild
}

// DoCommand uses a map string to run custom functionality of a fake powersensor.
func (f *PowerSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// Voltage gets the voltage and isAC of a fake powersensor.
func (f *PowerSensor) Voltage(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 1, true, nil
}

// Current gets the current and isAC of a fake powersensor.
func (f *PowerSensor) Current(ctx context.Context, cmd map[string]interface{}) (float64, bool, error) {
	return 3.5, true, nil
}

// Power gets the power of a fake powersensor.
func (f *PowerSensor) Power(ctx context.Context, cmd map[string]interface{}) (float64, error) {
	return 9.8, nil
}

// Readings gets the readings of a fake powersensor.
func (f *PowerSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return powersensor.Readings(ctx, f, extra)
}

// Start starts the fake powersensor.
func (f *PowerSensor) Start(ctx context.Context) error { return nil }

// Close closes the fake powersensor.
func (f *PowerSensor) Close(ctx context.Context) error {
	return nil
}
