// Package combouser implements a generic component that depends on a single composite
// (multi-API) resource and consumes every one of its APIs. Because combouser lives in a
// different module than the composite it depends on, the dependency handle it receives is a
// real composite client assembled over gRPC — exercising the composite path end to end rather
// than an in-process Go handle.
package combouser

import (
	"context"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/examples/customresources/apis/summationapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// Model is the full model definition of the consumer.
var Model = resource.NewModel("acme", "demo", "combouser")

// Config points at the composite resource this consumer depends on.
type Config struct {
	Combo string `json:"combo"`
}

// Validate ensures `combo` is set and declares it as a dependency by its bare name. The bare name
// resolves to the one composite resource regardless of which of its APIs we later pull off it.
func (cfg *Config) Validate(path string) ([]string, []string, error) {
	if cfg.Combo == "" {
		return nil, nil, resource.NewConfigValidationFieldRequiredError(path, "combo")
	}
	return []string{cfg.Combo}, nil, nil
}

func init() {
	resource.RegisterComponent(generic.API, Model, resource.Registration[resource.Resource, *Config]{
		Constructor: newComboUser,
	})
}

type comboUser struct {
	resource.Named
	resource.TriviallyCloseable

	gizmo gizmoapi.Gizmo
	sum   summationapi.Summation
	sen   sensor.Sensor
}

func newComboUser(
	ctx context.Context, deps resource.Dependencies, conf resource.Config, logger logging.Logger,
) (resource.Resource, error) {
	u := &comboUser{Named: conf.ResourceName().AsNamed()}
	if err := u.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return u, nil
}

// Reconfigure pulls a typed handle for each of the composite's APIs out of the single dependency.
// All three lookups target the same bare name; each FromProvider selects the right interface off
// the composite handle.
func (u *comboUser) Reconfigure(_ context.Context, deps resource.Dependencies, conf resource.Config) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	gizmo, err := resource.FromProvider[gizmoapi.Gizmo](deps, gizmoapi.Named(cfg.Combo))
	if err != nil {
		return err
	}
	sum, err := resource.FromProvider[summationapi.Summation](deps, summationapi.Named(cfg.Combo))
	if err != nil {
		return err
	}
	sen, err := resource.FromProvider[sensor.Sensor](deps, sensor.Named(cfg.Combo))
	if err != nil {
		return err
	}

	u.gizmo, u.sum, u.sen = gizmo, sum, sen
	return nil
}

// DoCommand exercises all three of the composite's APIs through the one dependency and returns
// what each reported, proving a single composite handle serves every API.
func (u *comboUser) DoCommand(ctx context.Context, _ map[string]interface{}) (map[string]interface{}, error) {
	two, err := u.gizmo.DoTwo(ctx, true)
	if err != nil {
		return nil, err
	}
	total, err := u.sum.Sum(ctx, []float64{1, 2, 3, 4})
	if err != nil {
		return nil, err
	}
	readings, err := u.sen.Readings(ctx, nil)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"gizmo_dotwo": two,
		"sum":         total,
		"readings":    readings,
	}, nil
}
