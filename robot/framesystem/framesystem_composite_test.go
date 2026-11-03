package framesystem

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
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

// modelFramerRes implements framesystem.ModelFramer but not InputEnabled: it has a model, but is not
// actuatable.
type modelFramerRes struct {
	resource.Named
	resource.TriviallyCloseable
}

func (modelFramerRes) Kinematics(context.Context) (referenceframe.Model, error) { return nil, nil }

// shapedRes implements resource.Shaped: it carries geometry but no model.
type shapedRes struct {
	resource.Named
	resource.TriviallyCloseable
}

func (shapedRes) Geometries(context.Context, map[string]interface{}) ([]spatialmath.Geometry, error) {
	return nil, nil
}

// TestBuiltInReconfigureComposite covers how BuiltInReconfigure folds a composite's per-API subs (sharing
// a short name) into svc.components, the name→sub map CurrentInputs reads. It unwraps to each API's real
// impl so the kinematic sub is found (else CurrentInputs fails NotInputEnabledError), keeps that sub even
// when its API is non-canonical, and refuses a multi-kinematic composite rather than crashing. The shared
// short name alone never errors.
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

	// No error on the duplicate short names.
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

// TestPickKinematicSubIdentity checks that pickKinematicSub distinguishes a composite by its InputEnabled
// impls' identity, not its kinematic API count. One struct served under several kinematic APIs is the same
// object each time (one chain, kept); distinct per-API implementations are the unsupported multi-kinematic
// case.
func TestPickKinematicSubIdentity(t *testing.T) {
	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	// One struct served under both kinematic APIs, the same object unwraps once per API.
	shared := &kinematicRes{Named: resource.NewName(armAPI, "shared").AsNamed()}
	chosen, refuse := pickKinematicSub([]resource.Resource{shared, shared})
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, chosen, test.ShouldEqual, shared)

	// Two distinct kinematic impls, the unsupported multi-kinematic device.
	kinArm := &kinematicRes{Named: resource.NewName(armAPI, "twochain").AsNamed()}
	kinGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "twochain").AsNamed()}
	_, refuse = pickKinematicSub([]resource.Resource{kinArm, kinGantry})
	test.That(t, refuse, test.ShouldBeTrue)
}

// TestFrameContribution checks the helper that resolves a resource's frame contribution across every API
// it serves: its kinematic (InputEnabled), model (ModelFramer), and shaped (geometry) impls, any of which
// may be nil, plus refuse for a multi-kinematic composite.
func TestFrameContribution(t *testing.T) {
	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// Ordinary kinematic resource: its own kinematic impl, and a ModelFramer too, but not shaped.
	arm := &kinematicRes{Named: resource.NewName(armAPI, "arm").AsNamed()}
	ie, model, shaped, refuse := FrameContribution(arm)
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, ie, test.ShouldEqual, arm)
	test.That(t, model, test.ShouldEqual, arm)
	test.That(t, shaped, test.ShouldBeNil)

	// Ordinary resource implementing none of the frame interfaces: all nil.
	ie, model, shaped, refuse = FrameContribution(&plainRes{Named: resource.NewName(camAPI, "cam").AsNamed()})
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, ie, test.ShouldBeNil)
	test.That(t, model, test.ShouldBeNil)
	test.That(t, shaped, test.ShouldBeNil)

	// ModelFramer that is not InputEnabled: a model, no kinematic or shaped impl.
	mf := &modelFramerRes{Named: resource.NewName(camAPI, "mf").AsNamed()}
	ie, model, shaped, refuse = FrameContribution(mf)
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, ie, test.ShouldBeNil)
	test.That(t, model, test.ShouldEqual, mf)
	test.That(t, shaped, test.ShouldBeNil)

	// Shaped resource: a geometry, no kinematic or model impl.
	shp := &shapedRes{Named: resource.NewName(camAPI, "shp").AsNamed()}
	ie, model, shaped, refuse = FrameContribution(shp)
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, ie, test.ShouldBeNil)
	test.That(t, model, test.ShouldBeNil)
	test.That(t, shaped, test.ShouldEqual, shp)

	// Composite spreading contributions across APIs: the kinematic (and model) impl is the non-canonical
	// gantry, the shaped impl is the canonical camera. Each is found whichever API exposes it.
	ncCam := &shapedRes{Named: resource.NewName(camAPI, "noncanon").AsNamed()}
	ncGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "noncanon").AsNamed()}
	ncWrapper := resource.NewMultiAPIResource(
		resource.NewName(camAPI, "noncanon"),
		map[resource.API]resource.Resource{camAPI: ncCam, gantryAPI: ncGantry},
	)
	ie, model, shaped, refuse = FrameContribution(ncWrapper)
	test.That(t, refuse, test.ShouldBeFalse)
	test.That(t, ie, test.ShouldEqual, ncGantry)
	test.That(t, model, test.ShouldEqual, ncGantry)
	test.That(t, shaped, test.ShouldEqual, ncCam)

	// Composite serving two distinct kinematic APIs: refused.
	multiArm := &kinematicRes{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	multiWrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "multi"),
		map[resource.API]resource.Resource{armAPI: multiArm, gantryAPI: multiGantry},
	)
	_, _, _, refuse = FrameContribution(multiWrapper)
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

	// Kept as one kinematic chain, the shared impl, not refused.
	test.That(t, svc.components["combo"], test.ShouldEqual, shared)
}
