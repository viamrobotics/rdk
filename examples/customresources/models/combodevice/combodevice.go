// Package combodevice implements a single model that serves three co-equal APIs at once — a
// custom component (acme:component:gizmo), a custom service (acme:service:summation), and a
// builtin component (rdk:component:sensor). It is registered with resource.RegisterMultiAPI, so
// one instance is one resource reachable under every one of those APIs.
package combodevice

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition of the composite device.
var Model = resource.NewModel("acme", "demo", "combodevice")

func init() {
	// RegisterMultiAPI registers the single constructor under every listed API and records the set,
	// marking this model as a composite. The first API (gizmo) is the canonical one used when a
	// config entry omits its `api` field. Panics on fewer than two APIs.
	resource.RegisterMultiAPI(
		[]resource.API{gizmoapi.API, summationapi.API, sensor.API},
		Model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: newComboDevice,
		},
	)
}

// comboDevice is one object that implements all three API interfaces: gizmoapi.Gizmo,
// summationapi.Summation, and sensor.Sensor.
type comboDevice struct {
	resource.Named
	resource.TriviallyReconfigurable
	resource.TriviallyCloseable
	logger logging.Logger
}

func newComboDevice(
	_ context.Context, _ resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	return &comboDevice{Named: conf.ResourceName().AsNamed(), logger: logger}, nil
}

// --- gizmoapi.Gizmo ---

func (c *comboDevice) DoOne(_ context.Context, arg1 string) (bool, error) {
	return arg1 == "combo", nil
}

func (c *comboDevice) DoOneClientStream(_ context.Context, arg1 []string) (bool, error) {
	ret := len(arg1) > 0
	for _, arg := range arg1 {
		ret = ret && arg == "combo"
	}
	return ret, nil
}

func (c *comboDevice) DoOneServerStream(_ context.Context, arg1 string) ([]bool, error) {
	return []bool{arg1 == "combo", false, true}, nil
}

func (c *comboDevice) DoOneBiDiStream(_ context.Context, arg1 []string) ([]bool, error) {
	var rets []bool
	for _, arg := range arg1 {
		rets = append(rets, arg == "combo")
	}
	return rets, nil
}

func (c *comboDevice) DoTwo(_ context.Context, arg1 bool) (string, error) {
	return fmt.Sprintf("combo saw arg1=%t", arg1), nil
}

// --- summationapi.Summation ---

func (c *comboDevice) Sum(_ context.Context, nums []float64) (float64, error) {
	var total float64
	for _, n := range nums {
		total += n
	}
	return total, nil
}

// --- sensor.Sensor ---

func (c *comboDevice) Readings(_ context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 42}, nil
}

// DoCommand satisfies resource.Resource (and gizmoapi.Gizmo's embedded DoCommand).
func (c *comboDevice) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return cmd, nil
}
