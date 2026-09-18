// Package main is a module demonstrating a composite: one model serving several co-equal APIs from a
// single identity. combosensor serves rdk:component:sensor, rdk:component:generic, and the custom
// acme:component:gizmo API — one instance, one config entry, reachable under each API.
package main

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "combosensor")

// wantArg is the gizmo argument the demo treats as a match; a named constant keeps the trivial demo
// logic in one place.
const wantArg = "combo"

func main() {
	// One registration declares the full co-equal API set; the single constructor is registered under
	// each. The custom gizmo API sits alongside the two builtin APIs to show that composites span
	// builtin and module-defined APIs alike.
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API, gizmoapi.API}, Model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newComboSensor,
		},
	)

	// ExpandModel reads back every API this Model was registered under, so main() never re-lists
	// them — the module advertises the composite on all of its APIs.
	module.ModularMain(resource.ExpandModel(Model)...)
}

func newComboSensor(
	_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
) (resource.Resource, error) {
	return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
}

// comboSensor implements sensor.Sensor (Readings), the generic component (DoCommand), and the custom
// gizmoapi.Gizmo interface from one identity. resource.Named supplies Name/DoCommand/Status; the
// embedded helpers supply Reconfigure/Close.
type comboSensor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

func (c *comboSensor) Readings(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

func (c *comboSensor) DoOne(_ context.Context, arg1 string) (bool, error) {
	return arg1 == wantArg, nil
}

func (c *comboSensor) DoOneClientStream(_ context.Context, arg1 []string) (bool, error) {
	for _, arg := range arg1 {
		if arg != wantArg {
			return false, nil
		}
	}
	return len(arg1) > 0, nil
}

func (c *comboSensor) DoOneServerStream(_ context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == wantArg, false}, nil
}

func (c *comboSensor) DoOneBiDiStream(_ context.Context, arg1 []string) ([]bool, error) {
	rets := make([]bool, 0, len(arg1))
	for _, arg := range arg1 {
		rets = append(rets, arg == wantArg)
	}
	return rets, nil
}

func (c *comboSensor) DoTwo(_ context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}
