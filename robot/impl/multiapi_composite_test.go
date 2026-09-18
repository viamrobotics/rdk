package robotimpl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot/client"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

// comboSensor serves both sensor.Sensor and generic.Resource from one identity.
type comboSensor struct {
	resource.Named
	resource.AlwaysRebuild
	closeCount *atomic.Int32
}

func (c *comboSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

func (c *comboSensor) Close(context.Context) error {
	if c.closeCount != nil {
		c.closeCount.Add(1)
	}
	return nil
}

// registerComboModel registers a fresh sensor+generic composite model and returns it, deregistering
// on test cleanup. Each Close on the built instance increments closeCount.
func registerComboModel(t *testing.T, name string, closeCount *atomic.Int32) resource.Model {
	t.Helper()
	model := resource.NewModel("acme", "test", name)
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed(), closeCount: closeCount}, nil
			},
		},
	)
	t.Cleanup(func() {
		resource.Deregister(sensor.API, model)
		resource.Deregister(generic.API, model)
	})
	return model
}

func TestCompositeResourceEndToEnd(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	var closeCount atomic.Int32
	model := registerComboModel(t, "composite-sensor", &closeCount)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model, Composite: true},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// 1) An api-less SimpleName resolves to the one handle; AsType extracts the sensor interface.
	res, err := r.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](res)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	// 2) A lookup by EACH co-equal API returns the SAME single instance as the api-less lookup.
	bySensor, err := r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, bySensor, test.ShouldEqual, res)

	byGeneric, err := r.ResourceByName(resource.NewName(generic.API, "combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byGeneric, test.ShouldEqual, res)

	// 3) In-process the api-less handle is the one raw instance, which natively implements every
	// co-equal API — AsType extracts each. (APIsOf on a raw builtin instance reports only its single
	// Name API; the modular composite handle, a resource.MultiAPIResource, reports the full set — see
	// TestModularCompositeResource.)
	_, err = resource.AsType[sensor.Sensor](res)
	test.That(t, err, test.ShouldBeNil)
	_, err = resource.AsType[resource.Resource](res)
	test.That(t, err, test.ShouldBeNil)

	// 4) It is advertised as N same-named ResourceNames (one per co-equal API).
	var advertised []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			advertised = append(advertised, n.API)
		}
	}
	test.That(t, advertised, test.ShouldContain, sensor.API)
	test.That(t, advertised, test.ShouldContain, generic.API)

	// 5) Teardown closes the single underlying instance exactly once, even though it is reachable
	// under several APIs. Reconfigure to an empty config to remove it, then assert the count.
	r.Reconfigure(ctx, &config.Config{})
	test.That(t, closeCount.Load(), test.ShouldEqual, 1)
}

// TestCompositeSimpleNameCollisionErrors asserts that a genuine same-name collision across distinct
// resources (NOT a composite) still errors on an api-less lookup — no regression from composite
// dedup.
func TestCompositeSimpleNameCollisionErrors(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	sensorModel := resource.NewModel("acme", "test", "collide-sensor")
	resource.RegisterComponent(sensor.API, sensorModel,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (sensor.Sensor, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(sensor.API, sensorModel)

	genericModel := resource.NewModel("acme", "test", "collide-generic")
	resource.RegisterComponent(generic.API, genericModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, genericModel)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "dup", API: sensor.API, Model: sensorModel},
			{Name: "dup", API: generic.API, Model: genericModel},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// Two DISTINCT resources share the simple name "dup"; an api-less lookup must not silently pick
	// one — it errors.
	_, err := r.ResourceByName(resource.SimpleName("dup"))
	test.That(t, err, test.ShouldNotBeNil)

	// A fully qualified lookup still resolves each one.
	_, err = r.ResourceByName(sensor.Named("dup"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(resource.NewName(generic.API, "dup"))
	test.That(t, err, test.ShouldBeNil)
}

// TestCompositeDependencyResolvesAllAPIs asserts that a second resource depending on a composite can
// resolve it under each of the composite's co-equal API names (the composite dependency is keyed
// under each API in the dependent's Dependencies).
func TestCompositeDependencyResolvesAllAPIs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	comboModel := registerComboModel(t, "composite-sensor-dep", nil)

	// A consumer that fails construction unless it can resolve the composite under BOTH APIs.
	consumerModel := resource.NewModel("acme", "test", "composite-consumer")
	resource.RegisterComponent(generic.API, consumerModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				if _, err := resource.FromProvider[sensor.Sensor](deps, sensor.Named("combo")); err != nil {
					return nil, fmt.Errorf("consumer could not resolve composite via sensor API: %w", err)
				}
				if _, err := resource.FromProvider[resource.Resource](deps, generic.Named("combo")); err != nil {
					return nil, fmt.Errorf("consumer could not resolve composite via generic API: %w", err)
				}
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, consumerModel)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: comboModel, Composite: true},
			{Name: "consumer", API: generic.API, Model: consumerModel, DependsOn: []string{"combo"}},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// If the consumer built, it resolved the composite under both APIs.
	_, err := r.ResourceByName(generic.Named("consumer"))
	test.That(t, err, test.ShouldBeNil)
}

func TestCompositeResourceOverClient(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-client", nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model, Composite: true},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)

	rc, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	// 1) advertised as N same-named ResourceNames over the wire.
	var apis []resource.API
	for _, n := range rc.ResourceNames() {
		if n.Name == "combo" {
			apis = append(apis, n.API)
		}
	}
	test.That(t, apis, test.ShouldContain, sensor.API)
	test.That(t, apis, test.ShouldContain, generic.API)

	// 2) client assembles an api-less composite handle; AsType extracts the sensor sub-client and the
	// call round-trips to the server instance.
	res, err := resource.NamedFromProvider(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](res)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	// 3) per-API access over the wire also works (typed FromRobot-style path).
	res2, err := rc.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	s2, err := resource.AsType[sensor.Sensor](res2)
	test.That(t, err, test.ShouldBeNil)
	r2, err := s2.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2["reading"], test.ShouldEqual, 7)
}

// TestCompositeRemoteResource asserts a composite on a remote is reachable under each of its APIs
// through the main robot.
func TestCompositeRemoteResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-remote", nil)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model, Composite: true},
		},
	}
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote.StartWeb(ctx, options), test.ShouldBeNil)

	mainCfg := &config.Config{
		Remotes: []config.Remote{{Name: "rem", Address: addr}},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// Reachable under both co-equal APIs through the main robot's remote.
	bySensor, err := main.ResourceByName(sensor.Named("rem:combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](bySensor)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	byGeneric, err := main.ResourceByName(resource.NewName(generic.API, "rem:combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byGeneric, test.ShouldNotBeNil)
}

func TestModularCompositeResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")

	model := resource.NewModel("acme", "demo", "combosensor")
	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "combo-mod", ExePath: modPath},
		},
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model, Composite: true},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// 1) api-less composite handle; AsType extracts the sensor sub-client; the call round-trips to the
	// module process.
	res, err := r.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](res)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	// The modular composite handle is a resource.MultiAPIResource, so APIsOf reports its full set.
	apisOf := resource.APIsOf(res)
	test.That(t, apisOf, test.ShouldContain, sensor.API)
	test.That(t, apisOf, test.ShouldContain, generic.API)

	// 2) reachable under its other builtin API too (one instance in the module, served under both).
	byGeneric, err := r.ResourceByName(resource.NewName(generic.API, "combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byGeneric, test.ShouldNotBeNil)

	// 3) advertised as N same-named ResourceNames.
	var apis []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			apis = append(apis, n.API)
		}
	}
	test.That(t, apis, test.ShouldContain, sensor.API)
	test.That(t, apis, test.ShouldContain, generic.API)
	test.That(t, apis, test.ShouldContain, gizmoapi.API)
}

// TestModularCompositeCustomAPI exercises the custom (module-defined) gizmo API of a composite over a
// client: the gizmo subtype server resolves the composite via the web resource getter, which unwraps
// it to the gizmo sub-resource, and the method round-trips to the module instance.
func TestModularCompositeCustomAPI(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")

	model := resource.NewModel("acme", "demo", "combosensor")
	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "combo-mod", ExePath: modPath},
		},
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model, Composite: true},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)

	rc, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	res, err := rc.ResourceByName(gizmoapi.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	giz, err := resource.AsType[gizmoapi.Gizmo](res)
	test.That(t, err, test.ShouldBeNil)

	ok, err := giz.DoOne(ctx, "combo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeTrue)

	ok, err = giz.DoOne(ctx, "nope")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, ok, test.ShouldBeFalse)
}
