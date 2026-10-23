package robotimpl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

// comboSensor serves both sensor.Sensor and generic.Resource from one identity.
type comboSensor struct {
	resource.Named
	closeCount *atomic.Int32
}

func (c *comboSensor) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

// DoCommand echoes cmd["ping"] back under "echo". It gives the composite's non-sensor (generic)
// implementation a method with an observable result, so a call can be proven to route to the correct
// per-API implementation of the one composite (rather than to Readings).
func (c *comboSensor) DoCommand(_ context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"echo": cmd["ping"]}, nil
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
			{Name: "combo", API: sensor.API, Model: model},
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

	// 2) An API-qualified lookup returns that API's handle directly (the unwrapped sub, not the
	// wrapper), usable as that API with no further unwrapping.
	bySensor, err := r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	s2, ok := bySensor.(sensor.Sensor)
	test.That(t, ok, test.ShouldBeTrue)
	readings, err = s2.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	byGeneric, err := r.ResourceByName(resource.NewName(generic.API, "combo"))
	test.That(t, err, test.ShouldBeNil)
	echoed, err := byGeneric.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, echoed["echo"], test.ShouldEqual, "pong")

	// 3) The api-less handle is the composite wrapper, uniform with a modular composite. APIsOf reports
	// the full co-equal set (not just the config API) and AsType extracts each served API.
	test.That(t, resource.APIsOf(res), test.ShouldContain, sensor.API)
	test.That(t, resource.APIsOf(res), test.ShouldContain, generic.API)
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

// TestCompositeModelReconfigureReindexes reconfigures a resource from a single-API model to a composite
// model serving an extra co-equal API (same name and config API). A model change rebuilds the resource
// in place on the same graph node, which does not pass through the graph's cache-write paths, so the
// manager must refresh the composite co-equal index afterward. Without that, the newly-served API would
// not resolve even though the resource now serves it.
func TestCompositeModelReconfigureReindexes(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// singleModel serves only sensor; multiModel serves sensor + generic from the same impl.
	singleModel := resource.NewModel("acme", "test", "recfg-single")
	resource.Register(sensor.API, singleModel, resource.Registration[resource.Resource, resource.NoNativeConfig]{
		Constructor: func(
			_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
		) (resource.Resource, error) {
			return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
		},
	})
	defer resource.Deregister(sensor.API, singleModel)
	multiModel := registerComboModel(t, "recfg-multi", nil)

	// Start as a plain sensor, so the generic API does not resolve.
	r := setupLocalRobot(t, ctx, &config.Config{
		Components: []resource.Config{{Name: "combo", API: sensor.API, Model: singleModel}},
	}, logger)
	_, err := r.ResourceByName(generic.Named("combo"))
	test.That(t, err, test.ShouldNotBeNil)

	// Reconfigure to the composite model (in-place rebuild on the same node). The generic API must now
	// resolve, and the configured sensor API must still resolve.
	r.Reconfigure(ctx, &config.Config{
		Components: []resource.Config{{Name: "combo", API: sensor.API, Model: multiModel}},
	})
	_, err = r.ResourceByName(generic.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
}

// TestCompositeDependencyResolvesAllAPIs asserts that a second resource depending on a composite can
// resolve it under each of the composite's co-equal API names (the composite dependency is keyed
// under each API in the dependent's Dependencies).
func TestCompositeDependencyResolvesAllAPIs(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	comboModel := registerComboModel(t, "composite-sensor-dep", nil)

	// A consumer that fails construction unless it can resolve the composite under both APIs.
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
			{Name: "combo", API: sensor.API, Model: comboModel},
			// Depend on the composite via an implicit dependency (DependsOn is deprecated).
			{Name: "consumer", API: generic.API, Model: consumerModel, ImplicitDependsOn: []string{"combo"}},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// If the consumer built, it resolved the composite under both APIs.
	_, err := r.ResourceByName(generic.Named("consumer"))
	test.That(t, err, test.ShouldBeNil)
}

// TestCompositeDependsOnComposite asserts a composite resource can itself depend on another composite,
// resolving it under each of its co-equal APIs, and is in turn advertised under all of its own.
func TestCompositeDependsOnComposite(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	innerModel := registerComboModel(t, "inner-combo", nil)

	// An outer composite (sensor + generic) whose constructor fails unless it resolves the inner composite
	// under both of its APIs.
	outerModel := resource.NewModel("acme", "test", "outer-combo")
	resource.RegisterMultiAPI(
		[]resource.API{sensor.API, generic.API}, outerModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				if _, err := resource.FromProvider[sensor.Sensor](deps, sensor.Named("inner")); err != nil {
					return nil, fmt.Errorf("outer could not resolve inner composite via sensor API: %w", err)
				}
				if _, err := resource.FromProvider[resource.Resource](deps, generic.Named("inner")); err != nil {
					return nil, fmt.Errorf("outer could not resolve inner composite via generic API: %w", err)
				}
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		},
	)
	defer resource.Deregister(sensor.API, outerModel)
	defer resource.Deregister(generic.API, outerModel)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "inner", API: sensor.API, Model: innerModel},
			{Name: "outer", API: sensor.API, Model: outerModel, ImplicitDependsOn: []string{"inner"}},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// outer built (so it resolved inner under both APIs) and is itself reachable under both of its APIs.
	_, err := r.ResourceByName(sensor.Named("outer"))
	test.That(t, err, test.ShouldBeNil)
	_, err = r.ResourceByName(generic.Named("outer"))
	test.That(t, err, test.ShouldBeNil)
}

// TestCompositeRemovalTearsDownDependent asserts removing a composite that a live resource depends on
// tears that dependent down (its required dep is gone) and drops the composite under every API.
func TestCompositeRemovalTearsDownDependent(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	comboModel := registerComboModel(t, "removal-combo", nil)

	consumerModel := resource.NewModel("acme", "test", "removal-consumer")
	resource.RegisterComponent(generic.API, consumerModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				if _, err := resource.FromProvider[sensor.Sensor](deps, sensor.Named("combo")); err != nil {
					return nil, fmt.Errorf("consumer could not resolve composite: %w", err)
				}
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, consumerModel)

	consumerCfg := resource.Config{
		Name: "consumer", API: generic.API, Model: consumerModel, ImplicitDependsOn: []string{"combo"},
	}
	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: comboModel},
			consumerCfg,
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)
	_, err := r.ResourceByName(generic.Named("consumer"))
	test.That(t, err, test.ShouldBeNil)

	// Remove the composite, keeping the dependent configured.
	r.Reconfigure(ctx, &config.Config{Components: []resource.Config{consumerCfg}})

	// The composite is gone under every API, and the dependent is no longer available.
	_, err = r.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = r.ResourceByName(generic.Named("combo"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = r.ResourceByName(generic.Named("consumer"))
	test.That(t, err, test.ShouldNotBeNil)
}

// TestCompositeRemoteResourceCollisions asserts that genuine collisions involving remote resources are
// detected. A local resource and a remote resource sharing a simple name collide, even across different
// APIs, as do two different (unprefixed) remotes exposing the same simple name.
func TestCompositeRemoteResourceCollisions(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	// Plain single-API models.
	sensorModel := resource.NewModel("acme", "test", "collide-remote-sensor")
	resource.RegisterComponent(sensor.API, sensorModel,
		resource.Registration[sensor.Sensor, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (sensor.Sensor, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(sensor.API, sensorModel)

	genericModel := resource.NewModel("acme", "test", "collide-remote-generic")
	resource.RegisterComponent(generic.API, genericModel,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				return &comboSensor{Named: conf.ResourceName().AsNamed()}, nil
			},
		})
	defer resource.Deregister(generic.API, genericModel)

	// Remote 1: a generic "gizmo" (will collide cross-API with the main robot's local sensor "gizmo")
	// and a sensor "widget" (will collide same-API with remote 2's sensor "widget").
	remote1Cfg := &config.Config{
		Components: []resource.Config{
			{Name: "gizmo", API: generic.API, Model: genericModel},
			{Name: "widget", API: sensor.API, Model: sensorModel},
		},
	}
	remote1 := setupLocalRobot(t, ctx, remote1Cfg, logger.Sublogger("remote1"))
	opts1, _, addr1 := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote1.StartWeb(ctx, opts1), test.ShouldBeNil)

	remote2Cfg := &config.Config{
		Components: []resource.Config{
			{Name: "widget", API: sensor.API, Model: sensorModel},
		},
	}
	remote2 := setupLocalRobot(t, ctx, remote2Cfg, logger.Sublogger("remote2"))
	opts2, _, addr2 := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote2.StartWeb(ctx, opts2), test.ShouldBeNil)

	// The main robot has a local sensor "gizmo" (collides cross-API with remote1's generic gizmo), and
	// two unprefixed remotes both exposing sensor "widget" (collide with each other).
	mainCfg := &config.Config{
		Components: []resource.Config{
			{Name: "gizmo", API: sensor.API, Model: sensorModel},
		},
		Remotes: []config.Remote{
			{Name: "r1", Address: addr1},
			{Name: "r2", Address: addr2},
		},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// Both genuine collisions are reported. The cross-API local-vs-remote "gizmo" (missed by a per-API
	// check) and the same-API remote-vs-remote "widget" both collide.
	testutils.WaitForAssertion(t, func(tb testing.TB) {
		test.That(tb, logs.FilterMessageSnippet("Resource name collision with a remote resource").Len(),
			test.ShouldBeGreaterThanOrEqualTo, 2)
	})

	// The local resource still wins a fully qualified lookup of its own name+API...
	localGizmo, err := main.ResourceByName(sensor.Named("gizmo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, localGizmo, test.ShouldNotBeNil)
	// ...and the remote resource is still reachable under its own remote-qualified name+API.
	remoteGizmo, err := main.ResourceByName(resource.NewName(generic.API, "r1:gizmo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, remoteGizmo, test.ShouldNotBeNil)
	// But an api-less lookup of the colliding simple name has no single owner (a local and a remote of a
	// different API claim it), so it is an error, preserving name uniqueness.
	_, err = main.ResourceByName(resource.SimpleName("gizmo"))
	test.That(t, err, test.ShouldNotBeNil)
}

// gripperServoDevice is the shared impl behind an in-process composite serving gripper.Gripper and
// servo.Servo. The per-API implementations below embed one *gripperServoDevice, so both route to this
// one instance (e.g. Stop increments the shared counter).
type gripperServoDevice struct {
	resource.Named
	stopCount  *atomic.Int32
	closeCount *atomic.Int32
}

func (d *gripperServoDevice) Close(context.Context) error {
	if d.closeCount != nil {
		d.closeCount.Add(1)
	}
	return nil
}

func (d *gripperServoDevice) Stop(context.Context, map[string]interface{}) error {
	if d.stopCount != nil {
		d.stopCount.Add(1)
	}
	return nil
}

func (d *gripperServoDevice) IsMoving(context.Context) (bool, error) { return false, nil }

func (d *gripperServoDevice) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (d *gripperServoDevice) Kinematics(context.Context) (referenceframe.Model, error) {
	return referenceframe.NewSimpleModel("combo"), nil
}

func (d *gripperServoDevice) CurrentInputs(context.Context) ([]referenceframe.Input, error) {
	return nil, nil
}

func (d *gripperServoDevice) GoToInputs(context.Context, ...[]referenceframe.Input) error { return nil }

type gripperFacade struct{ *gripperServoDevice }

func (f gripperFacade) Open(context.Context, map[string]interface{}) error { return nil }

func (f gripperFacade) Grab(context.Context, map[string]interface{}) (bool, error) { return false, nil }

func (f gripperFacade) IsHoldingSomething(
	context.Context, map[string]interface{},
) (gripper.HoldingStatus, error) {
	return gripper.HoldingStatus{}, nil
}

type servoFacade struct{ *gripperServoDevice }

func (f servoFacade) Move(context.Context, uint32, map[string]interface{}) error { return nil }

func (f servoFacade) Position(context.Context, map[string]interface{}) (uint32, error) { return 0, nil }

// registerGripperServoModel registers a builtin gripper+servo composite whose constructor composes the
// two implementations over one shared *gripperServoDevice. gripper sorts before servo, so gripper is the
// canonical API.
func registerGripperServoModel(t *testing.T, name string, stopCount, closeCount *atomic.Int32) resource.Model {
	t.Helper()
	model := resource.NewModel("acme", "test", name)
	resource.RegisterMultiAPI(
		[]resource.API{gripper.API, servo.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				d := &gripperServoDevice{Named: conf.ResourceName().AsNamed(), stopCount: stopCount, closeCount: closeCount}
				return resource.Compose(
					gripper.AsSub(gripperFacade{d}),
					servo.AsSub(servoFacade{d}),
				)
			},
		},
	)
	t.Cleanup(func() {
		resource.Deregister(gripper.API, model)
		resource.Deregister(servo.API, model)
	})
	return model
}

// TestCompositeInProcessTypedLookup checks that fetching an in-process composite by a specific API
// returns the concrete typed sub, so component.FromProvider(localRobot, name) works for each co-equal
// API, while an api-less lookup still returns the full composite.
func TestCompositeInProcessTypedLookup(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerGripperServoModel(t, "inproc-typed", nil, nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: gripper.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// In-process fetch by each specific API returns a working typed client.
	g, err := gripper.FromProvider(r, "combo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.Open(ctx, nil), test.ShouldBeNil)

	s, err := servo.FromProvider(r, "combo")
	test.That(t, err, test.ShouldBeNil)
	_, err = s.Position(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	// A specific-API ResourceByName returns the concrete sub (passes a plain type assertion), while the
	// api-less handle stays the composite serving every API.
	byGripper, err := r.ResourceByName(gripper.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	_, isGripper := byGripper.(gripper.Gripper)
	test.That(t, isGripper, test.ShouldBeTrue)

	one, err := r.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	apisOf := resource.APIsOf(one)
	test.That(t, apisOf, test.ShouldContain, gripper.API)
	test.That(t, apisOf, test.ShouldContain, servo.API)
}

// TestCompositeStopAllInProcess checks that StopAll reaches every actuator API a composite serves, not
// just its canonical one.
func TestCompositeStopAllInProcess(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	var stopCount atomic.Int32
	model := registerGripperServoModel(t, "inproc-stopall", &stopCount, nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: gripper.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	test.That(t, r.StopAll(ctx, nil), test.ShouldBeNil)
	// Every actuator API is stopped (safety). The gripper and servo facades are distinct objects, so
	// each is stopped; a repeat Stop on a shared core is safe because Stop must be idempotent.
	test.That(t, stopCount.Load(), test.ShouldEqual, 2)
}

// TestModularCompositeResource exercises the combomodule, a composite serving camera.Camera and
// movementsensor.MovementSensor from one module identity where the two APIs have colliding method
// names. Both declare a Properties method with different return types. camera sorts before
// movement_sensor, so camera is canonical.
func TestModularCompositeResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/complexcombomodule")

	model := resource.NewModel("acme", "demo", "combodevice")
	cfg := &config.Config{
		Modules: []config.Module{
			{Name: "combo-mod", ExePath: modPath},
		},
		Components: []resource.Config{
			{Name: "combo", API: camera.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// 1) api-less composite handle; AsType extracts each colliding API's sub-client.
	res, err := r.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	_, err = resource.AsType[camera.Camera](res)
	test.That(t, err, test.ShouldBeNil)
	_, err = resource.AsType[movementsensor.MovementSensor](res)
	test.That(t, err, test.ShouldBeNil)
	_, err = resource.AsType[gizmoapi.Gizmo](res)
	test.That(t, err, test.ShouldBeNil)

	// The modular composite handle is a resource.MultiAPIResource, so APIsOf reports its full set —
	// the two colliding builtin APIs plus the custom gizmo API.
	apisOf := resource.APIsOf(res)
	test.That(t, apisOf, test.ShouldContain, camera.API)
	test.That(t, apisOf, test.ShouldContain, movementsensor.API)
	test.That(t, apisOf, test.ShouldContain, gizmoapi.API)

	// 2) reachable under its other APIs too (one instance in the module, served under all of them).
	byMS, err := r.ResourceByName(movementsensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byMS, test.ShouldNotBeNil)

	byGiz, err := r.ResourceByName(gizmoapi.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, byGiz, test.ShouldNotBeNil)

	// 3) advertised as N same-named ResourceNames.
	var apis []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			apis = append(apis, n.API)
		}
	}
	test.That(t, apis, test.ShouldContain, camera.API)
	test.That(t, apis, test.ShouldContain, movementsensor.API)
	test.That(t, apis, test.ShouldContain, gizmoapi.API)

	// 4) Remove the composite (keep the module up). This tears down its module sub-clients. The composite
	// wrapper's Close closes every distinct sub-client, so none leak goroutines that this package's goleak
	// check flags at teardown. Assert the resource is gone under every API.
	r.Reconfigure(ctx, &config.Config{
		Modules: []config.Module{{Name: "combo-mod", ExePath: modPath}},
	})
	for _, n := range []resource.Name{
		resource.SimpleName("combo"), camera.Named("combo"), movementsensor.Named("combo"), gizmoapi.Named("combo"),
	} {
		_, err := r.ResourceByName(n)
		test.That(t, err, test.ShouldNotBeNil)
	}
}
