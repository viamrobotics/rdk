package robotimpl

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	gizmoapi "go.viam.com/rdk/examples/customresources/apis/gizmoapi"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
)

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
