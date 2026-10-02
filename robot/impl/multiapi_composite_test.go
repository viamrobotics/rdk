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
	"go.viam.com/rdk/robot"
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
// facade a method with an observable result, so a call can be proven to route to the correct per-API
// facade of the one composite (rather than to Readings).
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

	// 3) The api-less handle is the composite wrapper, uniform with a modular composite: APIsOf reports
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

	// Start as a plain sensor: the generic API does not resolve.
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

// TestCompositeRemoteResource asserts a composite hosted behind a remote resolves to ONE composite on
// the main robot — reachable under each co-equal API (method calls route to the right per-API facade),
// assembled into one api-less handle over the per-API remote sub-clients, and never mistaken for a
// name collision — across a direct remote, a prefixed remote, and a multi-hop remote chain. The graph
// flattens a chain to the immediate hop, so a nested composite lands on main just like a direct one.
func TestCompositeRemoteResource(t *testing.T) {
	newRemote := func(t *testing.T, ctx context.Context, logger logging.Logger, name string, model resource.Model) string {
		remote := setupLocalRobot(t, ctx, &config.Config{
			Components: []resource.Config{{Name: "combo", API: sensor.API, Model: model}},
		}, logger.Sublogger(name))
		opts, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
		test.That(t, remote.StartWeb(ctx, opts), test.ShouldBeNil)
		return addr
	}

	for _, tc := range []struct {
		name       string
		setup      func(t *testing.T, ctx context.Context, logger logging.Logger, model resource.Model) robot.LocalRobot
		perAPIName string   // remote-qualified name for the per-API lookups on main
		apiless    []string // every api-less form that must resolve to the one composite
	}{
		{
			name: "direct",
			setup: func(t *testing.T, ctx context.Context, logger logging.Logger, model resource.Model) robot.LocalRobot {
				addr := newRemote(t, ctx, logger, "remote", model)
				return setupLocalRobot(t, ctx, &config.Config{
					Remotes: []config.Remote{{Name: "rem", Address: addr}},
				}, logger.Sublogger("main"))
			},
			perAPIName: "rem:combo",
			apiless:    []string{"rem:combo"},
		},
		{
			// A remote resource is cached under its PREFIXED simple name, so combo behind a remote with
			// Prefix "p_" is "p_combo" on main; the api-less assembly must look it up under that name.
			name: "prefixed",
			setup: func(t *testing.T, ctx context.Context, logger logging.Logger, model resource.Model) robot.LocalRobot {
				addr := newRemote(t, ctx, logger, "remote", model)
				return setupLocalRobot(t, ctx, &config.Config{
					Remotes: []config.Remote{{Name: "rem", Address: addr, Prefix: "p_"}},
				}, logger.Sublogger("main"))
			},
			perAPIName: "rem:p_combo",
			apiless:    []string{"rem:p_combo", "p_combo"},
		},
		{
			// A chain main -> mid -> leaf: the graph flattens to the immediate hop (midrem), so the
			// composite collapses to one just like a direct remote, and every chain name form resolves.
			name: "nested",
			setup: func(t *testing.T, ctx context.Context, logger logging.Logger, model resource.Model) robot.LocalRobot {
				leafAddr := newRemote(t, ctx, logger, "leaf", model)
				mid := setupLocalRobot(t, ctx, &config.Config{
					Remotes: []config.Remote{{Name: "leafrem", Address: leafAddr}},
				}, logger.Sublogger("mid"))
				midOpts, _, midAddr := robottestutils.CreateBaseOptionsAndListener(t)
				test.That(t, mid.StartWeb(ctx, midOpts), test.ShouldBeNil)
				return setupLocalRobot(t, ctx, &config.Config{
					Remotes: []config.Remote{{Name: "midrem", Address: midAddr}},
				}, logger.Sublogger("main"))
			},
			perAPIName: "midrem:combo",
			apiless:    []string{"combo", "midrem:combo", "midrem:leafrem:combo"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logger, logs := logging.NewObservedTestLogger(t)
			ctx := context.Background()
			model := registerComboModel(t, "composite-sensor-remote-"+tc.name, nil)
			main := tc.setup(t, ctx, logger, model)

			// Reachable under both co-equal APIs; each API's method call routes to the correct facade.
			bySensor, err := main.ResourceByName(sensor.Named(tc.perAPIName))
			test.That(t, err, test.ShouldBeNil)
			s, err := resource.AsType[sensor.Sensor](bySensor)
			test.That(t, err, test.ShouldBeNil)
			readings, err := s.Readings(ctx, nil)
			test.That(t, err, test.ShouldBeNil)
			test.That(t, readings["reading"], test.ShouldEqual, 7)

			byGeneric, err := main.ResourceByName(resource.NewName(generic.API, tc.perAPIName))
			test.That(t, err, test.ShouldBeNil)
			echoed, err := byGeneric.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
			test.That(t, err, test.ShouldBeNil)
			test.That(t, echoed["echo"], test.ShouldEqual, "pong")

			// Every api-less form assembles the per-API sub-clients into ONE handle serving every API.
			for _, name := range tc.apiless {
				composite, err := main.ResourceByName(resource.SimpleName(name))
				test.That(t, err, test.ShouldBeNil)
				test.That(t, resource.APIsOf(composite), test.ShouldContain, sensor.API)
				test.That(t, resource.APIsOf(composite), test.ShouldContain, generic.API)
				cs, err := resource.AsType[sensor.Sensor](composite)
				test.That(t, err, test.ShouldBeNil)
				cReadings, err := cs.Readings(ctx, nil)
				test.That(t, err, test.ShouldBeNil)
				test.That(t, cReadings["reading"], test.ShouldEqual, 7)
			}

			// The composite's own per-API names must NOT be flagged as a name collision.
			test.That(t, logs.FilterMessageSnippet("Resource name collision").Len(), test.ShouldEqual, 0)
		})
	}
}

// TestCompositeRemoteMachineStatusSingleRow asserts a remote composite reports ONE MachineStatus row,
// matching a local composite. The remote proxies it as one node per co-equal API, so without deduping
// it would surface once per API while a local composite surfaces once.
func TestCompositeRemoteMachineStatusSingleRow(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-remote-status", nil)

	remote := setupLocalRobot(t, ctx, &config.Config{
		Components: []resource.Config{{Name: "combo", API: sensor.API, Model: model}},
	}, logger.Sublogger("remote"))
	opts, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote.StartWeb(ctx, opts), test.ShouldBeNil)

	main := setupLocalRobot(t, ctx, &config.Config{
		Remotes: []config.Remote{{Name: "rem", Address: addr}},
	}, logger.Sublogger("main"))

	// Sanity: the remote composite is reachable under both co-equal APIs on main (it IS proxied as two
	// per-API nodes), so a single status row is the result of deduping, not of the composite being absent.
	_, err := main.ResourceByName(sensor.Named("rem:combo"))
	test.That(t, err, test.ShouldBeNil)
	_, err = main.ResourceByName(resource.NewName(generic.API, "rem:combo"))
	test.That(t, err, test.ShouldBeNil)

	ms, err := main.MachineStatus(ctx)
	test.That(t, err, test.ShouldBeNil)

	var comboRows []resource.Name
	for _, rs := range ms.Resources {
		if rs.Name.Name == "combo" {
			comboRows = append(comboRows, rs.Name)
		}
	}
	test.That(t, len(comboRows), test.ShouldEqual, 1)
}

// TestCompositeRemoteResourceCollisions asserts that genuine collisions involving remote resources are
// detected: a local resource and a remote resource sharing a bare name collide -- even across
// DIFFERENT APIs -- as do two different (unprefixed) remotes exposing the same bare name.
func TestCompositeRemoteResourceCollisions(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	// Plain single-API models (NOT composites).
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

	// Main: a local sensor "gizmo" (collides cross-API with remote1's generic gizmo), and two
	// UNPREFIXED remotes both exposing sensor "widget" (collide with each other).
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

	// Both genuine collisions are reported: the cross-API local-vs-remote "gizmo" (missed by a per-API
	// check) and the same-API remote-vs-remote "widget".
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
	// But an api-less lookup of the colliding bare name has no single owner (a local AND a remote of a
	// different API claim it), so it is an error -- name uniqueness is preserved.
	_, err = main.ResourceByName(resource.SimpleName("gizmo"))
	test.That(t, err, test.ShouldNotBeNil)
}

// gripperServoDevice is the shared impl of an in-process facade composite serving gripper.Gripper and
// servo.Servo (both resource.Actuators; gripper is also resource.Shaped and framesystem.InputEnabled).
// All lifecycle/actuator state lives here, and the per-API facades below embed one *gripperServoDevice,
// so both facades' Stop route to this one Stop and increment the shared counter. It is assembled via
// resource.Compose, so it is stored as a resource.MultiAPIResource wrapper that does not itself satisfy
// those interfaces — an in-process fetch by a specific API must unwrap it to that API's sub, which is
// what these tests exercise.
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

// registerGripperServoModel registers a builtin gripper+servo colliding composite whose constructor
// assembles the two facades over one shared *gripperServoDevice via resource.Compose (so it is stored as a
// resource.MultiAPIResource wrapper). gripper sorts before servo, so gripper is the canonical API.
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
					conf.ResourceName(),
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

// TestModularCompositeInProcessTypedLookup checks in-process typed lookup of a facade composite: a
// composite stored as a resource.MultiAPIResource wrapper, fetched by a SPECIFIC API through the local
// (in-process) robot, yields the concrete typed sub-resource — so <component>.FromProvider(localRobot,
// name) returns a working typed client for each co-equal API. An api-less lookup still resolves to the
// full composite.
func TestModularCompositeInProcessTypedLookup(t *testing.T) {
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

	// A specific-API ResourceByName returns the concrete sub (passes a bare type assertion), while the
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

// TestCompositeStopAllInProcess checks that StopAll stops a modular/facade composite and does so
// exactly once: the composite serves two actuator APIs over one underlying instance, and StopAll's
// dedup stops that instance a single time, not once per advertised API.
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
	// Stopped AT ALL (safety) and exactly ONCE (dedup), not once per advertised API.
	test.That(t, stopCount.Load(), test.ShouldEqual, 1)
}

// TestCompositeInFrameSystem checks that a kinematic composite (canonical API gripper) is included in
// the frame system via its kinematic sub: the gripper sub is framesystem.InputEnabled, so the
// composite is added to the frame system with a model frame.
func TestCompositeInFrameSystem(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerGripperServoModel(t, "inproc-fs", nil, nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{
				Name:  "combo",
				API:   gripper.API,
				Model: model,
				Frame: &referenceframe.LinkConfig{Parent: referenceframe.World},
			},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	fsCfg, err := r.FrameSystemConfig(ctx)
	test.That(t, err, test.ShouldBeNil)
	var found bool
	for _, part := range fsCfg.Parts {
		if part.FrameConfig != nil && part.FrameConfig.Name() == "combo" {
			found = true
			// Included via the InputEnabled (kinematic) path, so it carries a model.
			test.That(t, part.ModelFrame, test.ShouldNotBeNil)
		}
	}
	test.That(t, found, test.ShouldBeTrue)
}

// TestModularCompositeResource exercises the combomodule, a composite serving camera.Camera and
// movementsensor.MovementSensor from one module identity where the two APIs have colliding method
// names. Both declare a Properties method with different return types. camera sorts before
// movement_sensor, so camera is canonical.
func TestModularCompositeResource(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")

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

	// 4) Remove the composite (keep the module up) — this tears down its module sub-clients. The
	// composite wrapper's Close reaches only the canonical sub, so modmanager.RemoveResource must close
	// the non-canonical per-API sub-clients too; if it does not they leak goroutines that this
	// package's goleak check flags at teardown. Assert the resource is gone under every API.
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
