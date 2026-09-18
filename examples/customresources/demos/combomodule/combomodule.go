// Package main is a module demonstrating a composite: one model serving several co-equal APIs from a
// single identity. combosensor serves both rdk:component:sensor and rdk:component:generic — one
// instance, one config entry, reachable under each API.
package main

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "combosensor")

func main() {
	// One registration declares the full API set; the single constructor is registered under each.
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API}, Model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newComboSensor,
		},
	)

	// APIModelsFor reads back every API this Model was registered under, so main() never re-lists
	// them — the module advertises the composite on all of its APIs.
	module.ModularMain(resource.APIModelsFor(Model)...)
}

func newComboSensor(
	_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
) (resource.Resource, error) {
	return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
}

// comboSensor implements both sensor.Sensor (Readings) and the generic component (DoCommand) from one
// identity. resource.Named supplies Name/DoCommand/Status; the embedded helpers supply
// Reconfigure/Close.
type comboSensor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

func (c *comboSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}
