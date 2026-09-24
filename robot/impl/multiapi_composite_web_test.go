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

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/pointcloud"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/robot/client"
	"go.viam.com/rdk/spatialmath"
	rtestutils "go.viam.com/rdk/testutils"
	"go.viam.com/rdk/testutils/robottestutils"
)

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

	// 3) per-API access over the wire also works (typed FromProvider-style path).
	res2, err := rc.ResourceByName(sensor.Named("combo"))
	test.That(t, err, test.ShouldBeNil)
	s2, err := resource.AsType[sensor.Sensor](res2)
	test.That(t, err, test.ShouldBeNil)
	r2, err := s2.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, r2["reading"], test.ShouldEqual, 7)
}

// TestModularCompositeCollidingMethods is the modular end-to-end counterpart of the builtin colliding
// test: over a client, camera.FromProvider(...).Properties and movementsensor.FromProvider(...).Properties
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

	cam, err := camera.FromProvider(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromProvider(rc, "combo")
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
//
//nolint:dupl // near-dup of TestModularCompositeCustomAPIUnderBuiltinAPI; distinct configs/assertions.
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
	cam, err := camera.FromProvider(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromProvider(rc, "combo")
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
//
//nolint:dupl // near-dup of TestModularCompositeCustomAPI; distinct configs/assertions.
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
	cam, err := camera.FromProvider(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromProvider(rc, "combo")
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
	cam, err := camera.FromProvider(rc, "combo")
	test.That(t, err, test.ShouldBeNil)
	camProps, err := cam.Properties(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, camProps.FrameRate, test.ShouldEqual, 30)
	test.That(t, camProps.SupportsPCD, test.ShouldBeTrue)

	ms, err := movementsensor.FromProvider(rc, "combo")
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
