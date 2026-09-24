package module

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appv1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

// comboSensor serves both sensor.Sensor and generic.Resource from one identity.
type comboSensor struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

func (c *comboSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// genericOnly implements generic (DoCommand via Named) but NOT sensor.Sensor (no Readings).
type genericOnly struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
}

// assertCompositeImplementsAPIs is the generic per-composite check: for every API a composite model
// declares, the constructed instance must be resolvable to that API's registered interface. It uses
// each API's registered collection, whose Add type-checks against the interface (resource.AsType) —
// the same mechanism the module's construct-once fan-out uses at startup. It returns the first API
// the instance fails to implement, or nil.
func assertCompositeImplementsAPIs(res resource.Resource, apis []resource.API) error {
	for _, api := range apis {
		reg, ok := resource.LookupGenericAPIRegistration(api)
		if !ok || reg.MakeEmptyCollection == nil {
			continue
		}
		if err := reg.MakeEmptyCollection().Add(res.Name(), res); err != nil {
			return err
		}
	}
	return nil
}

// TestCompositeDeclarationGuard asserts a model served under more than one API must be declared via
// resource.RegisterMultiAPI; undeclared multi-API reuse is rejected while a single-API model and a
// properly declared composite pass.
func TestCompositeDeclarationGuard(t *testing.T) {
	ctor := func(
		_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
	) (resource.Resource, error) {
		return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
	}

	t.Run("undeclared multi-API reuse is rejected", func(t *testing.T) {
		model := resource.NewModel("acme", "test", "undeclared-combo")
		// Two independent registrations under different APIs — NOT RegisterMultiAPI.
		resource.RegisterComponent(sensor.API, model,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: ctor})
		resource.RegisterComponent(generic.API, model,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: ctor})
		defer resource.Deregister(sensor.API, model)
		defer resource.Deregister(generic.API, model)

		err := validateCompositeDeclaration(model)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldContainSubstring, "RegisterMultiAPI")
	})

	t.Run("RegisterMultiAPI passes", func(t *testing.T) {
		model := resource.NewModel("acme", "test", "declared-combo")
		resource.RegisterMultiAPI([]resource.API{sensor.API, generic.API}, model,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: ctor})
		defer resource.Deregister(sensor.API, model)
		defer resource.Deregister(generic.API, model)

		test.That(t, validateCompositeDeclaration(model), test.ShouldBeNil)
	})

	t.Run("single-API model passes", func(t *testing.T) {
		model := resource.NewModel("acme", "test", "single-api")
		resource.RegisterComponent(sensor.API, model,
			resource.Registration[resource.Resource, resource.NoNativeConfig]{Constructor: ctor})
		defer resource.Deregister(sensor.API, model)

		test.That(t, validateCompositeDeclaration(model), test.ShouldBeNil)
	})
}

// TestCompositeStartupValidation asserts the per-composite implementation check: a correctly
// implemented composite passes for every declared API, and a model that fails to implement one of
// its declared APIs is caught (this is what the module's startup fan-out surfaces as a fast, clear
// error).
func TestCompositeStartupValidation(t *testing.T) {
	name := sensor.Named("combo")

	t.Run("implements all declared APIs", func(t *testing.T) {
		res := &comboSensor{Named: name.AsNamed()}
		err := assertCompositeImplementsAPIs(res, []resource.API{sensor.API, generic.API})
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("missing an API's methods is caught", func(t *testing.T) {
		// genericOnly has no Readings, so it does not implement sensor.Sensor.
		res := &genericOnly{Named: name.AsNamed()}
		err := assertCompositeImplementsAPIs(res, []resource.API{sensor.API, generic.API})
		test.That(t, err, test.ShouldNotBeNil)
	})
}

// TestCompositeAddResourceRollback asserts that when a composite fails to implement one of its
// declared APIs, addResource rolls the composite back out of every collection it already reached
// before closing the resource — so no collection is left pointing at a failed resource.
func TestCompositeAddResourceRollback(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	model := resource.NewModel("acme", "test", "rollback-combo")
	// genericOnly implements generic.Resource but NOT sensor.Sensor (no Readings), so the fan-out to
	// the sensor collection fails after the generic collection has already been populated.
	resource.RegisterMultiAPI([]resource.API{generic.API, sensor.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &genericOnly{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, model)
	defer resource.Deregister(sensor.API, model)

	//nolint:usetesting
	test.That(t, os.Setenv(NoModuleParentEnvVar, "true"), test.ShouldBeNil)
	defer func() { test.That(t, os.Unsetenv(NoModuleParentEnvVar), test.ShouldBeNil) }()

	m, err := NewModule(ctx, filepath.Join(t.TempDir(), "rollback.sock"), logger)
	test.That(t, err, test.ShouldBeNil)
	defer m.Close(ctx)
	_, err = m.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)

	// Register both API collections for the composite model.
	test.That(t, m.AddModelFromRegistry(ctx, generic.API, model), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, sensor.API, model), test.ShouldBeNil)

	// AddResource under the generic API: the generic collection Add succeeds, then the sensor fan-out
	// fails the type-check and the whole add must abort.
	_, err = m.AddResource(ctx, &pb.AddResourceRequest{
		Config: &appv1.ComponentConfig{Name: "combo", Api: generic.API.String(), Model: model.String()},
	})
	test.That(t, err, test.ShouldNotBeNil)

	// Rollback: the generic collection (populated before the failure) must not retain the resource.
	_, err = m.collections[generic.API].Resource("combo")
	test.That(t, err, test.ShouldNotBeNil)
}
