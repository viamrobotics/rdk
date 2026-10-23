// Package main is a module demonstrating the simple composite case, one model serving two co-equal
// APIs whose method names do not collide. simplecombo serves rdk:component:sensor and
// rdk:component:button from a single identity. Because sensor (Readings) and button (Push) share no
// method names, one shared struct satisfies both interfaces directly, so there are no per-API facades
// here, unlike the complexcombomodule demo where camera and movement_sensor collide on Properties and
// each needs its own facade. resource.Compose still assembles the two per-API sub-resources into one
// composite advertised under both APIs.
package main

import (
	"context"

	"go.viam.com/rdk/components/button"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "simplecombo")

func main() {
	// One registration declares the full co-equal API set; the single constructor is registered under each.
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, button.API}, Model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newSimpleCombo,
		},
	)

	// ExpandModel reads back every API this Model was registered under, so main() never re-lists them.
	module.ModularMain(resource.ExpandModel(Model)...)
}

func newSimpleCombo(
	_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
) (resource.Resource, error) {
	d := &simpleCombo{Named: conf.ResourceName().AsNamed()}
	// Compose builds one composite from two per-API subs. Both subs are the same *simpleCombo, with no
	// colliding methods, one struct implements both sensor.Sensor and button.Button, so no facades are
	// needed.
	return resource.Compose(
		sensor.AsSub(d),
		button.AsSub(d),
	)
}

// simpleCombo holds every method of both APIs on one struct.
type simpleCombo struct {
	resource.Named
	resource.TriviallyCloseable
}

// sensor (rdk:component:sensor).

func (d *simpleCombo) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// button (rdk:component:button), method name distinct from sensor's, so they live on the same struct.

func (d *simpleCombo) Push(context.Context, map[string]interface{}) error {
	return nil
}
