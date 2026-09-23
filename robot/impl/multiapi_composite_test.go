package robotimpl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"
	goutils "go.viam.com/utils"
	"go.viam.com/utils/rpc"
	"go.viam.com/utils/testutils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/servo"
	"go.viam.com/rdk/config"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/spatialmath"
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

	// Under machine-wide name uniqueness (#6350) a genuine collision is not merely hidden from an
	// api-less lookup: both colliding resources are torn down (CollidingNames) and stay unreachable —
	// even by a fully qualified name+API lookup — until the config is fixed. The composite dedup does
	// not exempt them, since they are distinct resources (different *GraphNodes), not one composite.
	_, err = r.ResourceByName(sensor.Named("dup"))
	test.That(t, err, test.ShouldNotBeNil)
	_, err = r.ResourceByName(resource.NewName(generic.API, "dup"))
	test.That(t, err, test.ShouldNotBeNil)
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
			{Name: "combo", API: sensor.API, Model: model},
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
// through the main robot: the N same-named remote resources that differ only by API are recognized
// as ONE composite (no spurious name-collision), each API's method call routes correctly, and an
// api-less lookup assembles a single handle whose APIsOf reports every API.
func TestCompositeRemoteResource(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-remote", nil)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model},
		},
	}
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote.StartWeb(ctx, options), test.ShouldBeNil)

	mainCfg := &config.Config{
		Remotes: []config.Remote{{Name: "rem", Address: addr}},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// Reachable under both co-equal APIs through the main robot's remote, and a method call routes to
	// the correct per-API facade of the one remote composite.
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
	// The generic-API facade's DoCommand round-trips through the remote to the one composite instance.
	echoed, err := byGeneric.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, echoed["echo"], test.ShouldEqual, "pong")

	// An api-less lookup resolves the remote composite to ONE handle serving every API (assembled over
	// the per-API remote sub-clients), not a name collision.
	composite, err := main.ResourceByName(resource.SimpleName("rem:combo"))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resource.APIsOf(composite), test.ShouldContain, sensor.API)
	test.That(t, resource.APIsOf(composite), test.ShouldContain, generic.API)
	// The single handle still unwraps to a working per-API sub-client.
	cs, err := resource.AsType[sensor.Sensor](composite)
	test.That(t, err, test.ShouldBeNil)
	cReadings, err := cs.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, cReadings["reading"], test.ShouldEqual, 7)

	// The composite's own per-API names must NOT be flagged as a name collision anywhere. This
	// matches the canonical #6350 collision messages (logMsgLocalNameCollision /
	// logMsgRemoteNameCollision), both of which begin "Resource name collision".
	test.That(t, logs.FilterMessageSnippet("Resource name collision").Len(), test.ShouldEqual, 0)
}

// TestCompositeRemoteResourceCollisions asserts that making the remote name check composite-aware did
// NOT weaken genuine collision detection. A local resource and a remote resource that share a bare
// name still collide -- even across DIFFERENT APIs, the case a single-API check would have missed --
// and two different (unprefixed) remotes exposing the same bare name collide as well.
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

	// Both genuine collisions are reported via #6350's canonical remote-collision message: the
	// cross-API local-vs-remote "gizmo" (missed by a per-API check) and the same-API
	// remote-vs-remote "widget".
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

// TestCompositeRemoteResourceWithPrefix is TestCompositeRemoteResource with a non-empty remote
// Prefix. A remote resource is cached under its PREFIXED simple name (the remote's prefix + bare
// name), so the composite "combo" behind a remote configured with Prefix "p_" lives on the main robot
// under the name "p_combo". An api-less lookup must therefore be made under the PREFIXED name, and it
// must still assemble the per-API sub-clients into ONE handle -- the earlier code looked the
// sub-clients up under the prefix-stripped name and so failed to resolve a prefixed remote composite.
// The prefixed per-API lookups already worked; this asserts the api-less assembly, per-API method
// routing, and that the composite is not mistaken for a name collision.
func TestCompositeRemoteResourceWithPrefix(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-remote-prefix", nil)

	remoteCfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model},
		},
	}
	remote := setupLocalRobot(t, ctx, remoteCfg, logger.Sublogger("remote"))
	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, remote.StartWeb(ctx, options), test.ShouldBeNil)

	// The remote is mounted with a non-empty prefix, so its "combo" is named "p_combo" on main.
	mainCfg := &config.Config{
		Remotes: []config.Remote{{Name: "rem", Address: addr, Prefix: "p_"}},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// Per-API prefixed lookups resolve and each API's method routes to the correct facade of the one
	// remote composite.
	bySensor, err := main.ResourceByName(sensor.Named("rem:p_combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](bySensor)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	byGeneric, err := main.ResourceByName(resource.NewName(generic.API, "rem:p_combo"))
	test.That(t, err, test.ShouldBeNil)
	echoed, err := byGeneric.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, echoed["echo"], test.ShouldEqual, "pong")

	// An api-less lookup under the PREFIXED name resolves the prefixed remote composite to ONE handle
	// serving every API (assembled over the per-API remote sub-clients). Both the remote-qualified and
	// the bare prefixed forms work; the remote qualifier is only a routing hint and is not required.
	for _, simpleName := range []string{"rem:p_combo", "p_combo"} {
		composite, err := main.ResourceByName(resource.SimpleName(simpleName))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resource.APIsOf(composite), test.ShouldContain, sensor.API)
		test.That(t, resource.APIsOf(composite), test.ShouldContain, generic.API)
		cs, err := resource.AsType[sensor.Sensor](composite)
		test.That(t, err, test.ShouldBeNil)
		cReadings, err := cs.Readings(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cReadings["reading"], test.ShouldEqual, 7)
	}

	// The prefixed composite's own per-API names must NOT be flagged as a name collision.
	test.That(t, logs.FilterMessageSnippet("resource name collision").Len(), test.ShouldEqual, 0)
}

// TestCompositeRemoteResourceNested asserts a composite reached through a CHAIN of remotes (main ->
// mid -> leaf) still resolves to one composite. The main robot's graph flattens a remote chain to the
// IMMEDIATE remote hop (updateRemoteResourceNames rewrites each remote resource's Remote to the direct
// remote it was reached through), so a composite behind >1 hop lands on main under the immediate
// remote just like a direct-remote composite: its per-API sibling names share that one Remote and so
// collapse to a single composite, and the same-remote collision exemption (match.Remote ==
// remoteName.Name) already covers it. This is the multi-hop composite the harness can express; the
// remote chain beyond the immediate hop is not represented on main.
func TestCompositeRemoteResourceNested(t *testing.T) {
	logger, logs := logging.NewObservedTestLogger(t)
	ctx := context.Background()
	model := registerComboModel(t, "composite-sensor-remote-nested", nil)

	// Leaf robot actually hosts the composite.
	leafCfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: sensor.API, Model: model},
		},
	}
	leaf := setupLocalRobot(t, ctx, leafCfg, logger.Sublogger("leaf"))
	leafOpts, _, leafAddr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, leaf.StartWeb(ctx, leafOpts), test.ShouldBeNil)

	// Middle robot remotes the leaf; it exposes the composite as "leafrem:combo".
	midCfg := &config.Config{
		Remotes: []config.Remote{{Name: "leafrem", Address: leafAddr}},
	}
	mid := setupLocalRobot(t, ctx, midCfg, logger.Sublogger("mid"))
	midOpts, _, midAddr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, mid.StartWeb(ctx, midOpts), test.ShouldBeNil)

	// Main remotes the middle robot, so the composite is reached through two hops (midrem -> leafrem).
	mainCfg := &config.Config{
		Remotes: []config.Remote{{Name: "midrem", Address: midAddr}},
	}
	main := setupLocalRobot(t, ctx, mainCfg, logger.Sublogger("main"))

	// Reachable under both co-equal APIs through the two-hop remote, and a method call routes to the
	// correct per-API facade of the one composite living two hops away.
	bySensor, err := main.ResourceByName(sensor.Named("midrem:combo"))
	test.That(t, err, test.ShouldBeNil)
	s, err := resource.AsType[sensor.Sensor](bySensor)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 7)

	byGeneric, err := main.ResourceByName(resource.NewName(generic.API, "midrem:combo"))
	test.That(t, err, test.ShouldBeNil)
	echoed, err := byGeneric.DoCommand(ctx, map[string]interface{}{"ping": "pong"})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, echoed["echo"], test.ShouldEqual, "pong")

	// An api-less lookup resolves the two-hop remote composite to ONE handle serving every API. The
	// bare name, the immediate-remote-qualified form, and the full-chain form all resolve (the remote
	// qualifier is only a routing hint; resolution keys off the bare name).
	for _, simpleName := range []string{"combo", "midrem:combo", "midrem:leafrem:combo"} {
		composite, err := main.ResourceByName(resource.SimpleName(simpleName))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, resource.APIsOf(composite), test.ShouldContain, sensor.API)
		test.That(t, resource.APIsOf(composite), test.ShouldContain, generic.API)
		cs, err := resource.AsType[sensor.Sensor](composite)
		test.That(t, err, test.ShouldBeNil)
		cReadings, err := cs.Readings(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, cReadings["reading"], test.ShouldEqual, 7)
	}

	// The nested composite's own per-API names must NOT be flagged as a name collision anywhere.
	test.That(t, logs.FilterMessageSnippet("resource name collision").Len(), test.ShouldEqual, 0)
}

// TestModularCompositeResource exercises the rebuilt combomodule: a COLLIDING composite serving
// camera.Camera + movementsensor.MovementSensor (both declare Properties, with different return
// types) from one module identity. camera sorts before movement_sensor, so camera is canonical.
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

// TestModularCompositeCollidingMethods is the modular end-to-end counterpart of the builtin colliding
// test: over a client, camera.FromRobot(...).Properties and movementsensor.FromRobot(...).Properties
// each round-trip to the module process and return their own facade's value — the two colliding
// Properties methods route to the correct per-API sub-resource across the wire.
func TestModularCompositeCollidingMethods(t *testing.T) {
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

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)

	rc, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	cam, err := camera.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	msProps, err := ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, msProps.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, msProps.PositionSupported, test.ShouldBeTrue)
}

// connectToSeparateServer dials a robot client to a viam-server running in a separate process,
// retrying until the server is dialable. It mirrors the module integration tests' connect helper
// (force direct gRPC, sessions disabled — modules do not yet support sessions).
func connectToSeparateServer(t *testing.T, ctx context.Context, port int, logger logging.Logger) robot.Robot {
	t.Helper()
	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		dialCtx, dialCancel := context.WithTimeout(ctx, 2*time.Second)
		rc, err := client.New(dialCtx, fmt.Sprintf("localhost:%d", port), logger,
			client.WithDialOptions(rpc.WithForceDirectGRPC()),
			client.WithDisableSessions(),
		)
		dialCancel()
		if err == nil {
			return rc
		}
		select {
		case <-connectCtx.Done():
			t.Fatalf("could not connect to separate-process server: %v", err)
			return nil
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// TestModularCompositeCustomAPI exercises the CUSTOM (module-defined) gizmo API of the composite over
// a client whose server has NO typed gizmo subtype server. The server runs as a separate viam-server
// process, whose binary does not import the example gizmoapi package, so gizmo is not a registered
// API there — a gizmo method call cannot be dispatched to a typed subtype server and instead falls to
// web.go's foreignServiceHandler, which unwraps the composite (a resource.MultiAPIResource) to its
// gizmo foreign sub-resource before proxying the call to the module. That composite-unwrap branch of
// foreignServiceHandler is the code under test; a correct DoOne result proves it fired (there is no
// other route for gizmo). camera and movement_sensor ARE compiled into the server binary, so their
// colliding Properties calls route through their typed subtype servers and must still land on the
// correct per-API facade of the one composite.
func TestModularCompositeCustomAPI(t *testing.T) {
	logger, logObserver := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	// Precompile the combomodule so the separate-process server only needs to exec it.
	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")
	model := resource.NewModel("acme", "demo", "combodevice")

	var port int
	var success bool
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		p, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		port = p

		cfg := &config.Config{
			Modules: []config.Module{{Name: "combo-mod", ExePath: modPath}},
			Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
				BindAddress: fmt.Sprintf("localhost:%d", port),
			}},
			Components: []resource.Config{
				// Declared under the custom gizmo API so the composite's canonical graph node is the
				// gizmo one: the server surfaces a foreign API's reflect descriptor (needed to route a
				// foreign method) only for a composite's stored node, so declaring under gizmo is what
				// makes GizmoService reachable through foreignServiceHandler here. camera and
				// movement_sensor are typed builtin APIs served by their own subtype servers, so they
				// resolve to the same composite regardless of which API it is declared under.
				{Name: "combo", API: gizmoapi.API, Model: model},
			},
		}
		cfgFilename, err := robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)
		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() { test.That(t, server.Stop(), test.ShouldBeNil) }()
			break
		}
		logger.Infow("port in use, restarting on a new port", "port", port)
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc := connectToSeparateServer(t, ctx, port, logger)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	// Custom gizmo API: no typed gizmo server exists on the parent, so this call routes through
	// foreignServiceHandler's composite-unwrap path. DoOne("combo") returns true (the facade's
	// wantArg) and DoOne("nope") returns false — proving the call reaches the gizmo facade of the
	// composite in the module and not some other API's sub-resource.
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

	// The two colliding builtin APIs still route to their own facades over the same composite: camera
	// Properties (FrameRate 30) and movement_sensor Properties (a different return type, distinguished
	// by AngularVelocitySupported) must not cross-wire.
	cam, err := camera.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	msProps, err := ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, msProps.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, msProps.PositionSupported, test.ShouldBeTrue)
}

// TestModularCompositeCustomAPIUnderBuiltinAPI is the regression test for the ResourceRPCAPIs
// composite-expansion fix. It is identical to TestModularCompositeCustomAPI EXCEPT the composite is
// declared under a BUILTIN API (camera) rather than the custom gizmo API. The composite still serves
// the custom gizmo API co-equally, but its canonical graph node is now the camera one. Before the
// fix, ResourceRPCAPIs iterated only the raw graph names, so it emitted a descriptor for the
// canonical (camera) API alone and never advertised the co-equal gizmo API; a gizmo call then failed
// in the client with Unimplemented in TypeAndMethodDescFromMethod, before web.go's
// foreignServiceHandler composite-unwrap could ever fire. With the fix, ResourceRPCAPIs expands the
// composite so every co-equal API surfaces its RPC descriptor regardless of the declared API, and the
// gizmo call routes correctly. The server runs as a separate process whose binary does not import
// gizmoapi, so gizmo is reachable only via the foreign path.
func TestModularCompositeCustomAPIUnderBuiltinAPI(t *testing.T) {
	logger, logObserver := logging.NewObservedTestLogger(t)
	ctx := context.Background()

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")
	model := resource.NewModel("acme", "demo", "combodevice")

	var port int
	var success bool
	for portTryNum := 0; portTryNum < 10; portTryNum++ {
		p, err := goutils.TryReserveRandomPort()
		test.That(t, err, test.ShouldBeNil)
		port = p

		cfg := &config.Config{
			Modules: []config.Module{{Name: "combo-mod", ExePath: modPath}},
			Network: config.NetworkConfig{NetworkConfigData: config.NetworkConfigData{
				BindAddress: fmt.Sprintf("localhost:%d", port),
			}},
			Components: []resource.Config{
				// Declared under the BUILTIN camera API: the composite's canonical graph node is the
				// camera one, NOT the gizmo one. This is the case the fix targets — the co-equal gizmo
				// API must still be advertised (via ResourceRPCAPIs expansion) so the foreign gizmo call
				// can route through foreignServiceHandler.
				{Name: "combo", API: camera.API, Model: model},
			},
		}
		cfgFilename, err := robottestutils.MakeTempConfig(t, cfg, logger)
		test.That(t, err, test.ShouldBeNil)

		server := robottestutils.ServerAsSeparateProcess(t, cfgFilename, logger)
		err = server.Start(context.Background())
		test.That(t, err, test.ShouldBeNil)

		if success = robottestutils.WaitForServing(logObserver, port); success {
			defer func() { test.That(t, server.Stop(), test.ShouldBeNil) }()
			break
		}
		logger.Infow("port in use, restarting on a new port", "port", port)
		server.Stop()
	}
	test.That(t, success, test.ShouldBeTrue)

	rc := connectToSeparateServer(t, ctx, port, logger)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	// Custom gizmo API over a composite DECLARED UNDER camera: before the fix this failed with
	// Unimplemented because ResourceRPCAPIs never advertised the gizmo descriptor for a camera-declared
	// composite. DoOne("combo") returns true and DoOne("nope") returns false, proving the call reached
	// the gizmo facade of the composite in the module via foreignServiceHandler's unwrap path.
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

	// The two colliding builtin APIs still route to their own facades over the same composite.
	cam, err := camera.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	msProps, err := ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, msProps.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, msProps.PositionSupported, test.ShouldBeTrue)
}

// collideDevice is a builtin, in-process COLLIDING composite: one shared device serving camera.Camera
// and movementsensor.MovementSensor, whose Properties methods collide by name with different return
// types. Every non-colliding method and all lifecycle state (the close counter) live here; the two
// Properties methods are carried by the per-API facades below, which embed one *collideDevice. This
// mirrors the rebuilt combomodule but keeps the shared impl in the test process so Close-once is
// directly observable.
type collideDevice struct {
	resource.Named
	resource.AlwaysRebuild
	closeCount *atomic.Int32
}

func (d *collideDevice) Close(context.Context) error {
	if d.closeCount != nil {
		d.closeCount.Add(1)
	}
	return nil
}

func (d *collideDevice) Images(
	context.Context, []string, map[string]interface{},
) ([]camera.NamedImage, resource.ResponseMetadata, error) {
	return nil, resource.ResponseMetadata{}, nil
}

func (d *collideDevice) NextPointCloud(context.Context, map[string]interface{}) (pointcloud.PointCloud, error) {
	return nil, nil
}

func (d *collideDevice) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (d *collideDevice) Position(context.Context, map[string]interface{}) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, nil
}

func (d *collideDevice) LinearVelocity(context.Context, map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (d *collideDevice) AngularVelocity(context.Context, map[string]interface{}) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, nil
}

func (d *collideDevice) LinearAcceleration(context.Context, map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, nil
}

func (d *collideDevice) CompassHeading(context.Context, map[string]interface{}) (float64, error) {
	return 0, nil
}

func (d *collideDevice) Orientation(context.Context, map[string]interface{}) (spatialmath.Orientation, error) {
	return spatialmath.NewZeroOrientation(), nil
}

func (d *collideDevice) Accuracy(context.Context, map[string]interface{}) (*movementsensor.Accuracy, error) {
	return &movementsensor.Accuracy{}, nil
}

func (d *collideDevice) Readings(context.Context, map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"reading": 7}, nil
}

type camFacadeBuiltin struct{ *collideDevice }

func (f camFacadeBuiltin) Properties(context.Context) (camera.Properties, error) {
	return camera.Properties{SupportsPCD: true, FrameRate: 30}, nil
}

type imuFacadeBuiltin struct{ *collideDevice }

func (f imuFacadeBuiltin) Properties(context.Context, map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{PositionSupported: true, AngularVelocitySupported: true}, nil
}

// registerCollideModel registers a builtin camera+movementsensor colliding composite whose constructor
// assembles the two facades over one shared *collideDevice via resource.Compose. Deregistered on
// cleanup. Each Close on the shared device increments closeCount.
func registerCollideModel(t *testing.T, name string, closeCount *atomic.Int32) resource.Model {
	t.Helper()
	model := resource.NewModel("acme", "test", name)
	resource.RegisterMultiAPI(
		[]resource.API{camera.API, movementsensor.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				d := &collideDevice{Named: conf.ResourceName().AsNamed(), closeCount: closeCount}
				return resource.Compose(
					conf.ResourceName(),
					camera.AsSub(camFacadeBuiltin{d}),
					movementsensor.AsSub(imuFacadeBuiltin{d}),
				)
			},
		},
	)
	t.Cleanup(func() {
		resource.Deregister(camera.API, model)
		resource.Deregister(movementsensor.API, model)
	})
	return model
}

// TestCompositeCollidingMethodsBuiltin is the point of the whole feature: a builtin composite serving
// two APIs that COLLIDE on a method name (camera.Properties vs movementsensor.Properties, different
// return types). Over a client, each colliding Properties call must route to the correct per-API
// facade; the api-less handle resolves to the one composite; and the shared impl closes exactly once
// even though it is advertised under both APIs.
func TestCompositeCollidingMethodsBuiltin(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	var closeCount atomic.Int32
	// camera sorts before movement_sensor, so camera is the canonical (sorted-first) API.
	model := registerCollideModel(t, "collide-cam-imu", &closeCount)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: camera.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	options, _, addr := robottestutils.CreateBaseOptionsAndListener(t)
	test.That(t, r.StartWeb(ctx, options), test.ShouldBeNil)

	rc, err := client.New(ctx, addr, logger)
	test.That(t, err, test.ShouldBeNil)
	defer func() { test.That(t, rc.Close(ctx), test.ShouldBeNil) }()

	// The two colliding Properties methods each route to their own facade over the wire.
	cam, err := camera.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromRobot(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	msProps, err := ms.Properties(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, msProps.AngularVelocitySupported, test.ShouldBeTrue)
	test.That(t, msProps.PositionSupported, test.ShouldBeTrue)

	// An api-less SimpleName resolves to the one composite handle, which serves both APIs.
	one, err := rc.ResourceByName(resource.SimpleName("combo"))
	test.That(t, err, test.ShouldBeNil)
	apisOf := resource.APIsOf(one)
	test.That(t, apisOf, test.ShouldContain, camera.API)
	test.That(t, apisOf, test.ShouldContain, movementsensor.API)

	// Advertised as one same-named ResourceName per co-equal API on the server.
	var advertised []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			advertised = append(advertised, n.API)
		}
	}
	test.That(t, advertised, test.ShouldContain, camera.API)
	test.That(t, advertised, test.ShouldContain, movementsensor.API)

	// Teardown closes the single shared impl exactly once, though it is advertised under both APIs.
	r.Reconfigure(ctx, &config.Config{})
	test.That(t, closeCount.Load(), test.ShouldEqual, 1)
}

// stopKinDevice is a builtin, in-process facade composite's shared impl serving gripper.Gripper and
// servo.Servo. Both APIs are resource.Actuators, and gripper is additionally resource.Shaped and
// framesystem.InputEnabled. All lifecycle/actuator state lives here; the per-API facades below embed
// one *stopKinDevice, so both facades' Stop route to this one Stop and increment the shared counter.
// Because it is assembled via resource.Compose it is stored as a resource.MultiAPIResource wrapper —
// the wrapper implements none of these concrete interfaces, which is exactly the in-process gap the
// specific-API unwrap closes (a raw single-struct builtin implements them natively and dodges it).
type stopKinDevice struct {
	resource.Named
	resource.AlwaysRebuild
	stopCount  *atomic.Int32
	closeCount *atomic.Int32
}

func (d *stopKinDevice) Close(context.Context) error {
	if d.closeCount != nil {
		d.closeCount.Add(1)
	}
	return nil
}

func (d *stopKinDevice) Stop(context.Context, map[string]interface{}) error {
	if d.stopCount != nil {
		d.stopCount.Add(1)
	}
	return nil
}

func (d *stopKinDevice) IsMoving(context.Context) (bool, error) { return false, nil }

func (d *stopKinDevice) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

func (d *stopKinDevice) Kinematics(context.Context) (referenceframe.Model, error) {
	return referenceframe.NewSimpleModel("combo"), nil
}

func (d *stopKinDevice) CurrentInputs(context.Context) ([]referenceframe.Input, error) {
	return nil, nil
}

func (d *stopKinDevice) GoToInputs(context.Context, ...[]referenceframe.Input) error { return nil }

type gripperFacade struct{ *stopKinDevice }

func (f gripperFacade) Open(context.Context, map[string]interface{}) error { return nil }

func (f gripperFacade) Grab(context.Context, map[string]interface{}) (bool, error) { return false, nil }

func (f gripperFacade) IsHoldingSomething(
	context.Context, map[string]interface{},
) (gripper.HoldingStatus, error) {
	return gripper.HoldingStatus{}, nil
}

type servoFacade struct{ *stopKinDevice }

func (f servoFacade) Move(context.Context, uint32, map[string]interface{}) error { return nil }

func (f servoFacade) Position(context.Context, map[string]interface{}) (uint32, error) { return 0, nil }

// registerStopKinModel registers a builtin gripper+servo colliding composite whose constructor
// assembles the two facades over one shared *stopKinDevice via resource.Compose (so it is stored as a
// resource.MultiAPIResource wrapper). gripper sorts before servo, so gripper is the canonical API.
func registerStopKinModel(t *testing.T, name string, stopCount, closeCount *atomic.Int32) resource.Model {
	t.Helper()
	model := resource.NewModel("acme", "test", name)
	resource.RegisterMultiAPI(
		[]resource.API{gripper.API, servo.API}, model,
		resource.Registration[resource.Resource, resource.NoNativeConfig]{
			Constructor: func(
				_ context.Context, _ resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (resource.Resource, error) {
				d := &stopKinDevice{Named: conf.ResourceName().AsNamed(), stopCount: stopCount, closeCount: closeCount}
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

// TestModularCompositeInProcessTypedLookup is the core regression test for the in-process gap: a
// facade composite stored as a resource.MultiAPIResource wrapper, fetched by a SPECIFIC API through
// the LOCAL (in-process) robot, must yield the concrete typed sub-resource — the wrapper implements
// none of the API interfaces, so before the specific-API unwrap every <component>.FromRobot(localRobot,
// name) TypeErrored. The api-less handle still resolves to the full composite.
func TestModularCompositeInProcessTypedLookup(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerStopKinModel(t, "inproc-typed", nil, nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: gripper.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// In-process fetch by each specific API returns a working typed client (TypeErrored before the fix).
	g, err := gripper.FromRobot(r, "combo")
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.Open(ctx, nil), test.ShouldBeNil)

	s, err := servo.FromRobot(r, "combo")
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

// TestCompositeStopAllInProcess proves the safety fix: StopAll actually stops a modular/facade
// composite (before the unwrap the composite wrapper is not a resource.Actuator, so emergency-stop
// silently skipped it), and the StopAll dedup stops the one underlying instance exactly once even
// though the composite is advertised under two actuator APIs.
func TestCompositeStopAllInProcess(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	var stopCount atomic.Int32
	model := registerStopKinModel(t, "inproc-stopall", &stopCount, nil)

	cfg := &config.Config{
		Components: []resource.Config{
			{Name: "combo", API: gripper.API, Model: model},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	// The composite is advertised under both gripper and servo (both actuators).
	var advertised []resource.API
	for _, n := range r.ResourceNames() {
		if n.Name == "combo" {
			advertised = append(advertised, n.API)
		}
	}
	test.That(t, advertised, test.ShouldContain, gripper.API)
	test.That(t, advertised, test.ShouldContain, servo.API)

	test.That(t, r.StopAll(ctx, nil), test.ShouldBeNil)
	// Stopped AT ALL (safety) and exactly ONCE (dedup), not once per advertised API.
	test.That(t, stopCount.Load(), test.ShouldEqual, 1)
}

// TestCompositeInFrameSystem proves a kinematic composite (gripper canonical API) is included in the
// frame system. Before the unwrap the composite wrapper failed the framesystem.InputEnabled type
// assertion and was omitted; after it, the concrete gripper sub is InputEnabled and is included.
func TestCompositeInFrameSystem(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	model := registerStopKinModel(t, "inproc-fs", nil, nil)

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
