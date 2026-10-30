package module

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	appv1 "go.viam.com/api/app/v1"
	pb "go.viam.com/api/module/v1"
	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

// comboSensor serves both sensor.Sensor and generic.Resource from one identity.
type comboSensor struct {
	resource.Named
	resource.TriviallyCloseable
}

func (c *comboSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// genericOnly implements generic (DoCommand via Named) but not sensor.Sensor (no Readings).
type genericOnly struct {
	resource.Named
	resource.TriviallyCloseable
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
		// Two independent registrations under different APIs, not RegisterMultiAPI.
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

// TestCompositeAddResourceRollback asserts that when a composite fails to implement one of its
// declared APIs, addResource rolls the composite back out of every collection it already reached
// before closing the resource, so no collection is left pointing at a failed resource.
func TestCompositeAddResourceRollback(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	model := resource.NewModel("acme", "test", "rollback-combo")
	// genericOnly implements generic.Resource but not sensor.Sensor (no Readings), so the fan-out to
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
	test.That(t, err.Error(), test.ShouldContainSubstring, "does not implement declared api")
	test.That(t, err.Error(), test.ShouldContainSubstring, sensor.API.String())

	// On rollback, the generic collection (populated before the failure) must not retain the resource.
	_, err = m.collections[generic.API].Resource("combo")
	test.That(t, resource.IsNotFoundError(err), test.ShouldBeTrue)
}

// rtpCamComposite serves camera and sensor from one object; its camera side is an rtppassthrough.Source.
type rtpCamComposite struct {
	*inject.Camera
}

func (c *rtpCamComposite) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// TestCompositeCameraStreamSourceRegistered asserts a composite whose camera sub is an
// rtppassthrough.Source registers (and, on remove, clears) that stream source under its camera name.
func TestCompositeCameraStreamSourceRegistered(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	model := resource.NewModel("acme", "test", "rtp-combo")
	resource.RegisterMultiAPI([]resource.API{camera.API, sensor.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &rtpCamComposite{Camera: inject.NewCamera(conf.ResourceName().Name)}, nil
			},
		})
	defer resource.Deregister(camera.API, model)
	defer resource.Deregister(sensor.API, model)

	//nolint:usetesting
	test.That(t, os.Setenv(NoModuleParentEnvVar, "true"), test.ShouldBeNil)
	defer func() { test.That(t, os.Unsetenv(NoModuleParentEnvVar), test.ShouldBeNil) }()

	m, err := NewModule(ctx, filepath.Join(t.TempDir(), "rtp.sock"), logger)
	test.That(t, err, test.ShouldBeNil)
	defer m.Close(ctx)
	_, err = m.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.AddModelFromRegistry(ctx, camera.API, model), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, sensor.API, model), test.ShouldBeNil)

	_, err = m.AddResource(ctx, &pb.AddResourceRequest{
		Config: &appv1.ComponentConfig{Name: "cam", Api: camera.API.String(), Model: model.String()},
	})
	test.That(t, err, test.ShouldBeNil)

	// The stream source is registered under the camera API only, not under the composite's other co-equal
	// API (sensor is not a stream source).
	m.registerMu.Lock()
	_, underCamera := m.streamSourceByName[camera.Named("cam")]
	_, underSensor := m.streamSourceByName[sensor.Named("cam")]
	m.registerMu.Unlock()
	test.That(t, underCamera, test.ShouldBeTrue)
	test.That(t, underSensor, test.ShouldBeFalse)

	// Removing the composite clears the stream source.
	_, err = m.RemoveResource(ctx, &pb.RemoveResourceRequest{Name: camera.Named("cam").String()})
	test.That(t, err, test.ShouldBeNil)
	m.registerMu.Lock()
	_, stillRegistered := m.streamSourceByName[camera.Named("cam")]
	m.registerMu.Unlock()
	test.That(t, stillRegistered, test.ShouldBeFalse)
}

// localConsumer depends on a composite in the same module and remembers the exact instance it resolved.
type localConsumer struct {
	resource.Named
	resource.TriviallyCloseable
	dep resource.Resource
}

// TestCompositeRebuildUpdatesInModuleDependent reconfigures a modular composite that a second resource in
// the same module depends on, and checks the dependent is cascaded onto the rebuilt instance rather than
// left holding the stale, closed one.
func TestCompositeRebuildUpdatesInModuleDependent(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	var built int
	compModel := resource.NewModel("acme", "test", "rebuild-combo")
	resource.RegisterMultiAPI([]resource.API{generic.API, sensor.API}, compModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger) (resource.Resource, error) {
				built++
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, compModel)
	defer resource.Deregister(sensor.API, compModel)

	consumerModel := resource.NewModel("acme", "test", "combo-consumer")
	resource.RegisterComponent(generic.API, consumerModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger) (resource.Resource, error) {
				c := &localConsumer{Named: conf.ResourceName().AsNamed()}
				for n, d := range deps {
					if n.Name == "combo" {
						c.dep = d
					}
				}
				return c, nil
			},
		})
	defer resource.Deregister(generic.API, consumerModel)

	//nolint:usetesting
	test.That(t, os.Setenv(NoModuleParentEnvVar, "true"), test.ShouldBeNil)
	defer func() { test.That(t, os.Unsetenv(NoModuleParentEnvVar), test.ShouldBeNil) }()

	m, err := NewModule(ctx, filepath.Join(t.TempDir(), "rebuild.sock"), logger)
	test.That(t, err, test.ShouldBeNil)
	defer m.Close(ctx)
	_, err = m.Ready(ctx, &pb.ReadyRequest{})
	test.That(t, err, test.ShouldBeNil)

	test.That(t, m.AddModelFromRegistry(ctx, generic.API, compModel), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, sensor.API, compModel), test.ShouldBeNil)
	test.That(t, m.AddModelFromRegistry(ctx, generic.API, consumerModel), test.ShouldBeNil)

	comboCfg := &appv1.ComponentConfig{Name: "combo", Api: sensor.API.String(), Model: compModel.String()}
	_, err = m.AddResource(ctx, &pb.AddResourceRequest{Config: comboCfg})
	test.That(t, err, test.ShouldBeNil)
	_, err = m.AddResource(ctx, &pb.AddResourceRequest{
		Config:       &appv1.ComponentConfig{Name: "consumer", Api: generic.API.String(), Model: consumerModel.String()},
		Dependencies: []string{sensor.Named("combo").String()},
	})
	test.That(t, err, test.ShouldBeNil)

	// The consumer resolved the composite locally (not a parent client).
	m.registerMu.Lock()
	consumer := m.localResources["consumer"].res.(*localConsumer)
	firstCombo := m.localResources["combo"].res
	m.registerMu.Unlock()
	test.That(t, consumer.dep, test.ShouldEqual, firstCombo)

	// Reconfiguring the composite rebuilds it; the in-module dependent must be cascaded onto the new one.
	_, err = m.ReconfigureResource(ctx, &pb.ReconfigureResourceRequest{Config: comboCfg})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, built, test.ShouldEqual, 2)

	m.registerMu.Lock()
	consumer = m.localResources["consumer"].res.(*localConsumer)
	newCombo := m.localResources["combo"].res
	m.registerMu.Unlock()
	test.That(t, newCombo, test.ShouldNotEqual, firstCombo)
	test.That(t, consumer.dep, test.ShouldEqual, newCombo)
	test.That(t, consumer.dep, test.ShouldNotEqual, firstCombo)
}
