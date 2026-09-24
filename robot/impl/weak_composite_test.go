package robotimpl

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.viam.com/test"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rtestutils "go.viam.com/rdk/testutils"
)

// TestWeakDependencyOnComposite asserts a composite is wired to a weak-dependent under EACH of its
// co-equal APIs the matcher matches — as the unwrapped per-API sub — not only its canonical API. The
// combomodule composite is configured under camera (its graph/canonical API), so before the fix a
// component weak-matcher wired it only under camera; now it is also wired under movement_sensor.
func TestWeakDependencyOnComposite(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// A component that weak-depends on every component and records the deps it received.
	weakAPI := resource.NewAPI(uuid.NewString(), "component", "weakcomposite")
	weakModel := resource.DefaultModelFamily.WithModel(utils.RandomAlphaString(5))
	weakName := resource.NewName(weakAPI, "weak1")
	resource.Register(weakAPI, weakModel,
		resource.Registration[*someTypeWithWeakAndStrongDeps, *someTypeWithWeakAndStrongDepsConfig]{
			Constructor: func(
				_ context.Context, deps resource.Dependencies, conf resource.Config, _ logging.Logger,
			) (*someTypeWithWeakAndStrongDeps, error) {
				return &someTypeWithWeakAndStrongDeps{Named: conf.ResourceName().AsNamed(), resources: deps}, nil
			},
			WeakDependencies: []resource.Matcher{resource.TypeMatcher{Type: resource.APITypeComponentName}},
		})
	defer resource.Deregister(weakAPI, weakModel)

	modPath := rtestutils.BuildTempModule(t, "examples/customresources/demos/combomodule")
	model := resource.NewModel("acme", "demo", "combodevice")
	cfg := &config.Config{
		Modules: []config.Module{{Name: "combo-mod", ExePath: modPath}},
		Components: []resource.Config{
			{Name: "combo", API: camera.API, Model: model},
			{Name: weakName.Name, API: weakAPI, Model: weakModel},
		},
	}
	r := setupLocalRobot(t, ctx, cfg, logger)

	weakRes, err := r.ResourceByName(weakName)
	test.That(t, err, test.ShouldBeNil)
	weak, err := resource.AsType[*someTypeWithWeakAndStrongDeps](weakRes)
	test.That(t, err, test.ShouldBeNil)

	// Wired under camera (canonical) AND movement_sensor (non-canonical) — the latter is the fix.
	camDep, ok := weak.resources[camera.Named("combo")]
	test.That(t, ok, test.ShouldBeTrue)
	_, err = resource.AsType[camera.Camera](camDep)
	test.That(t, err, test.ShouldBeNil)

	msDep, ok := weak.resources[movementsensor.Named("combo")]
	test.That(t, ok, test.ShouldBeTrue)
	// It is the unwrapped movement_sensor sub, not the composite wrapper.
	_, err = resource.AsType[movementsensor.MovementSensor](msDep)
	test.That(t, err, test.ShouldBeNil)
}
