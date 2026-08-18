package modmanager

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	modmanageroptions "go.viam.com/rdk/module/modmanager/options"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
)

// TestMultiAPIModule verifies the modular path of composable APIs: a module advertises one model
// under two APIs (sensor + generic), and viam-server serves it as one instance reachable under both
// — the module constructs it once (module.Module.AddResource) and the manager returns one composite
// whose per-API sub-clients both route to that instance.
func TestMultiAPIModule(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	parentAddr := setupSocketWithRobot(t)
	modPath := rtestutils.BuildTempModule(t, "module/testmodule")

	mgr := setupModManager(t, ctx, parentAddr, logger, modmanageroptions.Options{UntrustedEnv: false})
	err := mgr.Add(ctx, config.Module{Name: "test-module", ExePath: modPath, Type: config.ModuleTypeLocal})
	test.That(t, err, test.ShouldBeNil)

	multiModel := resource.NewModel("rdk", "test", "multi")

	// The module advertised the model under two APIs, so viam-server recorded it as multi-API and a
	// config may omit `api` (resolved to the canonical, first-sorted API).
	apis := resource.APIsForModel(multiModel)
	test.That(t, apis, test.ShouldContain, sensor.API)
	test.That(t, apis, test.ShouldContain, generic.API)

	cfg := resource.Config{Name: "combo", API: sensor.API, Model: multiModel}
	_, _, err = cfg.Validate("test", resource.APITypeComponentName)
	test.That(t, err, test.ShouldBeNil)

	res, err := mgr.AddResource(ctx, cfg, nil)
	test.That(t, err, test.ShouldBeNil)

	// One composite object, usable under both APIs — each sub-client routes to the one module instance.
	s, err := sensor.FromResource(res)
	test.That(t, err, test.ShouldBeNil)
	readings, err := s.Readings(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, readings["reading"], test.ShouldEqual, 42)

	g, err := generic.FromResource(res)
	test.That(t, err, test.ShouldBeNil)
	resp, err := g.DoCommand(ctx, map[string]interface{}{})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, resp["did"], test.ShouldEqual, "it")

	// The resource is modular under both of its API names.
	test.That(t, mgr.IsModularResource(sensor.Named("combo")), test.ShouldBeTrue)
	test.That(t, mgr.IsModularResource(generic.Named("combo")), test.ShouldBeTrue)

	test.That(t, mgr.Close(ctx), test.ShouldBeNil)
}
