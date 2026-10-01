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
// (sharing a short name) into its short-name-keyed component map: it unwraps a MODULAR composite to
// this API's real sub — without which the kinematic sub is never detected and CurrentInputs later
// fails NotInputEnabledError — keeps the single kinematic sub, refuses a composite serving more than
// one kinematic API (unsupported) instead of crashing, and never errors on the duplicate short name.
// The kept kinematic sub need not be the canonical API.
func TestBuiltInReconfigureComposite(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	svcIface, err := New(ctx, resource.Dependencies{}, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := svcIface.(*frameSystemService)

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// "combo": a modular composite serving arm (kinematic) + camera (non-kinematic) — one
	// MultiAPIResource wrapper aliased under both API names.
	comboKin := &kinematicRes{Named: resource.NewName(armAPI, "combo").AsNamed()}
	comboCam := &plainRes{Named: resource.NewName(camAPI, "combo").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "combo"),
		[]resource.API{armAPI, camAPI},
		map[resource.API]resource.Resource{armAPI: comboKin, camAPI: comboCam},
	)
	// "multi": a composite serving two kinematic APIs (arm + gantry) — unsupported, must be refused.
	multiArm := &kinematicRes{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	// "solo": an ordinary resource.
	solo := &plainRes{Named: resource.NewName(camAPI, "solo").AsNamed()}
	// "noncanon": a composite whose KINEMATIC API is NOT the canonical one — camera (canonical,
	// non-kinematic) + gantry (kinematic). The kinematic sub must still be kept, independent of which
	// API sorts first.
	ncCam := &plainRes{Named: resource.NewName(camAPI, "noncanon").AsNamed()}
	ncGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "noncanon").AsNamed()}
	ncWrapper := resource.NewMultiAPIResource(
		resource.NewName(camAPI, "noncanon"),
		[]resource.API{camAPI, gantryAPI},
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

	// The modular composite is unwrapped to its kinematic (arm) sub — not the opaque wrapper — so
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
	// composite), not just when it is canonical.
	test.That(t, svc.components["noncanon"], test.ShouldEqual, ncGantry)
}

// TestKinematicSubIdentity checks that KinematicSub and MultiKinematic distinguish a composite by the
// identity of its InputEnabled subs, not by how many kinematic APIs it serves. A single struct composed
// under several kinematic APIs resolves to one shared object under each, so it is one kinematic chain
// and is kept; per-API facades are distinct objects, so two kinematic ones are the unsupported
// multi-kinematic case and are refused.
func TestKinematicSubIdentity(t *testing.T) {
	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	// One struct composed under both kinematic APIs: every API resolves to the same object.
	shared := &kinematicRes{Named: resource.NewName(armAPI, "shared").AsNamed()}
	oneChain := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "shared"),
		[]resource.API{armAPI, gantryAPI},
		map[resource.API]resource.Resource{armAPI: shared, gantryAPI: shared},
	)
	ie, ok := KinematicSub(oneChain)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, ie, test.ShouldEqual, shared)
	test.That(t, MultiKinematic(oneChain), test.ShouldBeFalse)

	// Two distinct kinematic subs: unsupported multi-kinematic device.
	kinArm := &kinematicRes{Named: resource.NewName(armAPI, "twochain").AsNamed()}
	kinGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "twochain").AsNamed()}
	twoChains := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "twochain"),
		[]resource.API{armAPI, gantryAPI},
		map[resource.API]resource.Resource{armAPI: kinArm, gantryAPI: kinGantry},
	)
	_, ok = KinematicSub(twoChains)
	test.That(t, ok, test.ShouldBeFalse)
	test.That(t, MultiKinematic(twoChains), test.ShouldBeTrue)
}

// TestBuiltInReconfigureSingleStructMultiKinematic verifies a single struct served under two kinematic
// APIs is KEPT as one chain by BuiltInReconfigure -- agreeing with KinematicSub/MultiKinematic and the
// model-frame builder. The composite wrapper is aliased under both API names and unwraps to the SAME
// object under each, so a count-based guard would see two kinematic subs and wrongly refuse it, leaving
// its frame modeled but svc.components["combo"] unset and CurrentInputs unserviceable machine-wide.
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
		[]resource.API{armAPI, gantryAPI},
		map[resource.API]resource.Resource{armAPI: shared, gantryAPI: shared},
	)
	deps := resource.Dependencies{
		resource.NewName(armAPI, "combo"):    wrapper,
		resource.NewName(gantryAPI, "combo"): wrapper,
	}

	err = svc.BuiltInReconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}})
	test.That(t, err, test.ShouldBeNil)

	// Kept as one kinematic chain (the shared sub), consistent with KinematicSub(wrapper) -- not refused.
	kin, ok := KinematicSub(wrapper)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, svc.components["combo"], test.ShouldEqual, shared)
	test.That(t, svc.components["combo"], test.ShouldEqual, kin)
}
