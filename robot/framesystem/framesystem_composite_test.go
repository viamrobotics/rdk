package framesystem

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

// kinematicRes implements framesystem.InputEnabled (a kinematic resource); plainRes does not.
type kinematicRes struct {
	resource.Named
	resource.TriviallyCloseable
}

func (kinematicRes) Kinematics(context.Context) (referenceframe.Model, error) { return nil, nil }

func (kinematicRes) CurrentInputs(context.Context) ([]referenceframe.Input, error) { return nil, nil }
func (kinematicRes) GoToInputs(context.Context, ...[]referenceframe.Input) error   { return nil }

type plainRes struct {
	resource.Named
	resource.TriviallyCloseable
}

// TestBuiltInReconfigureComposite covers how the frame system folds a composite's per-API siblings
// (sharing a short name) into its short-name-keyed component map. It unwraps the composite to each API's
// real impl so the kinematic sub is detected; otherwise CurrentInputs later fails NotInputEnabledError.
// It keeps the single kinematic sub, whose API need not be the canonical one, and refuses a composite
// serving more than one kinematic API as unsupported rather than crashing. The shared short name alone
// never errors.
func TestBuiltInReconfigureComposite(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	svcIface, err := New(ctx, resource.Dependencies{}, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := svcIface.(*frameSystemService)

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// "combo": a modular composite serving arm (kinematic) + camera (non-kinematic), one
	// MultiAPIResource wrapper aliased under both API names.
	comboKin := &kinematicRes{Named: resource.NewName(armAPI, "combo").AsNamed()}
	comboCam := &plainRes{Named: resource.NewName(camAPI, "combo").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "combo"),
		map[resource.API]resource.Resource{armAPI: comboKin, camAPI: comboCam},
	)
	// "multi": a composite serving two kinematic APIs (arm + gantry), unsupported, must be refused.
	multiArm := &kinematicRes{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	// "solo": an ordinary resource.
	solo := &plainRes{Named: resource.NewName(camAPI, "solo").AsNamed()}
	// "noncanon": a composite whose kinematic API is not the canonical one, camera (canonical,
	// non-kinematic) + gantry (kinematic). The kinematic sub must still be kept, independent of which
	// API sorts first.
	ncCam := &plainRes{Named: resource.NewName(camAPI, "noncanon").AsNamed()}
	ncGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "noncanon").AsNamed()}
	ncWrapper := resource.NewMultiAPIResource(
		resource.NewName(camAPI, "noncanon"),
		map[resource.API]resource.Resource{camAPI: ncCam, gantryAPI: ncGantry},
	)

	deps := resource.Dependencies{
		resource.NewName(armAPI, "combo"):       wrapper,
		resource.NewName(camAPI, "combo"):       wrapper,
		resource.NewName(armAPI, "multi"):       multiArm,
		resource.NewName(gantryAPI, "multi"):    multiGantry,
		resource.NewName(camAPI, "solo"):        solo,
		resource.NewName(camAPI, "noncanon"):    ncWrapper,
		resource.NewName(gantryAPI, "noncanon"): ncWrapper,
	}

	// No error on the duplicate short names (the old code returned DuplicateResourceNameError).
	err = svc.BuiltInReconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}})
	test.That(t, err, test.ShouldBeNil)

	// The modular composite is unwrapped to its kinematic (arm) sub, not the opaque wrapper, so
	// CurrentInputs can type-assert InputEnabled on it.
	test.That(t, svc.components["combo"], test.ShouldEqual, comboKin)
	_, isInputEnabled := svc.components["combo"].(InputEnabled)
	test.That(t, isInputEnabled, test.ShouldBeTrue)
	// A multi-kinematic composite is refused, not silently picked.
	_, present := svc.components["multi"]
	test.That(t, present, test.ShouldBeFalse)
	// An ordinary resource is unaffected.
	test.That(t, svc.components["solo"], test.ShouldEqual, solo)
	// The kinematic sub is kept even when it is the non-canonical API (gantry under a camera-canonical
	// composite).
	test.That(t, svc.components["noncanon"], test.ShouldEqual, ncGantry)
}

// TestPickKinematicSubIdentity checks that PickKinematicSub distinguishes a composite by its InputEnabled
// subs' identity, not its kinematic API count. One struct served under several kinematic APIs is the same
// object each time (one chain, kept); distinct per-API implementations are the unsupported multi-kinematic
// case.
func TestPickKinematicSubIdentity(t *testing.T) {
	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	// One struct served under both kinematic APIs, the same object unwraps once per API.
	shared := &kinematicRes{Named: resource.NewName(armAPI, "shared").AsNamed()}
	chosen, refuse := PickKinematicSub([]resource.Resource{shared, shared})
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, chosen, test.ShouldEqual, shared)

	// Two distinct kinematic subs, the unsupported multi-kinematic device.
	kinArm := &kinematicRes{Named: resource.NewName(armAPI, "twochain").AsNamed()}
	kinGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "twochain").AsNamed()}
	_, refuse = PickKinematicSub([]resource.Resource{kinArm, kinGantry})
	test.That(t, refuse, test.ShouldBeTrue)
}

// TestBuiltInReconfigureSingleStructMultiKinematic verifies a single struct served under two kinematic
// APIs is kept as one chain by BuiltInReconfigure. The composite wrapper is aliased under both API names
// and unwraps to the same object under each, so a count-based guard would see two kinematic subs and
// wrongly refuse it, leaving its frame modeled but svc.components["combo"] unset and CurrentInputs
// unserviceable.
func TestBuiltInReconfigureSingleStructMultiKinematic(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	svcIface, err := New(ctx, resource.Dependencies{}, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := svcIface.(*frameSystemService)

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	// One struct serving two kinematic APIs, aliased under each (the single-struct composite shape).
	shared := &kinematicRes{Named: resource.NewName(armAPI, "combo").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "combo"),
		map[resource.API]resource.Resource{armAPI: shared, gantryAPI: shared},
	)
	deps := resource.Dependencies{
		resource.NewName(armAPI, "combo"):    wrapper,
		resource.NewName(gantryAPI, "combo"): wrapper,
	}

	err = svc.BuiltInReconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}})
	test.That(t, err, test.ShouldBeNil)

	// Kept as one kinematic chain, the shared sub, not refused.
	test.That(t, svc.components["combo"], test.ShouldEqual, shared)
}
