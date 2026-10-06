// Package main is a module demonstrating the SIMPLE composite case: one model serving two co-equal
// APIs whose method names do NOT collide. simplecombo serves rdk:component:sensor and the custom
// acme:component:gizmo API from a single identity. Because sensor (Readings) and gizmo (DoOne/DoTwo/…)
// share no method names, one shared struct satisfies BOTH interfaces directly — there are no per-API
// facades here, unlike the combomodule demo where camera and movement_sensor collide on Properties and
// each needs its own facade. resource.Compose still assembles the two per-API sub-resources into one
// composite advertised under both APIs.
package main

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
)

// Model is registered under every API it serves via RegisterMultiAPI.
var Model = resource.NewModel("acme", "demo", "simplecombo")

// wantArg is the gizmo argument the demo treats as a match; a named constant keeps the trivial demo
// logic in one place.
const wantArg = "simplecombo"

func main() {
	// One registration declares the full co-equal API set; the single constructor is registered under
	// each. sensor and gizmo share no method names, so unlike combomodule there is nothing to route to
	// separate facades — this demo exists to show the composite machinery at its simplest.
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, gizmoapi.API}, Model,
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
	// Compose builds one composite from two per-API subs. Both subs are the SAME *simpleCombo — with no
	// colliding methods, one struct implements both sensor.Sensor and gizmoapi.Gizmo, so no facades are
	// needed. The composite is one device with one lifecycle, reachable under sensor and gizmo alike.
	return resource.Compose(
		conf.ResourceName(),
		sensor.AsSub(d),
		gizmoapi.AsSub(d),
	)
}

// simpleCombo holds every method of both APIs on one struct. resource.Named supplies Name/DoCommand and
// TriviallyCloseable supplies Close; the sensor and gizmo methods below complete both interfaces.
type simpleCombo struct {
	resource.Named
	resource.TriviallyCloseable
}

// sensor (rdk:component:sensor).

func (d *simpleCombo) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// gizmo (acme:component:gizmo) — names distinct from sensor's, so they live on the same struct.

func (d *simpleCombo) DoOne(_ context.Context, arg1 string) (bool, error) {
	return arg1 == wantArg, nil
}

func (d *simpleCombo) DoOneClientStream(_ context.Context, arg1 []string) (bool, error) {
	for _, arg := range arg1 {
		if arg != wantArg {
			return false, nil
		}
	}
	return len(arg1) > 0, nil
}

func (d *simpleCombo) DoOneServerStream(_ context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == wantArg, false}, nil
}

func (d *simpleCombo) DoOneBiDiStream(_ context.Context, arg1 []string) ([]bool, error) {
	rets := make([]bool, 0, len(arg1))
	for _, arg := range arg1 {
		rets = append(rets, arg == wantArg)
	}
	return rets, nil
}

func (d *simpleCombo) DoTwo(_ context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("arg1=%t", arg1), nil
}
