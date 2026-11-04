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

// TestBuiltInReconfigureComposite covers how BuiltInReconfigure maps each resource to its kinematic
// subresource. ResolveKinematic finds that subresource (so CurrentInputs can assert InputEnabled) even when
// its API is non-canonical, refuses a multi-kinematic composite, and gives a non-kinematic resource no
// entry. Duplicate short names across a composite's APIs never error.
func TestBuiltInReconfigureComposite(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	svcIface, err := New(ctx, resource.Dependencies{}, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := svcIface.(*frameSystemService)

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// "combo": a composite serving arm (kinematic) + camera (non-kinematic).
	comboKin := &kinematicRes{Named: resource.NewName(armAPI, "combo").AsNamed()}
	comboCam := &plainRes{Named: resource.NewName(camAPI, "combo").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "combo"),
		map[resource.API]resource.Resource{armAPI: comboKin, camAPI: comboCam},
	)
	// "multi": a composite serving two kinematic APIs (arm + gantry) with distinct objects, unsupported,
	// must be refused.
	multiArm := &kinematicRes{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	multiWrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "multi"),
		map[resource.API]resource.Resource{armAPI: multiArm, gantryAPI: multiGantry},
	)
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
		resource.NewName(armAPI, "multi"):       multiWrapper,
		resource.NewName(gantryAPI, "multi"):    multiWrapper,
		resource.NewName(camAPI, "solo"):        solo,
		resource.NewName(camAPI, "noncanon"):    ncWrapper,
		resource.NewName(gantryAPI, "noncanon"): ncWrapper,
	}

	// No error on the duplicate short names.
	err = svc.BuiltInReconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}})
	test.That(t, err, test.ShouldBeNil)

	// The composite is unwrapped to its kinematic (arm) sub, not the opaque wrapper, so
	// CurrentInputs can type-assert InputEnabled on it.
	test.That(t, svc.components["combo"], test.ShouldEqual, comboKin)
	_, isInputEnabled := svc.components["combo"].(InputEnabled)
	test.That(t, isInputEnabled, test.ShouldBeTrue)
	// A multi-kinematic composite is refused, not silently picked.
	_, present := svc.components["multi"]
	test.That(t, present, test.ShouldBeFalse)
	// A non-kinematic ordinary resource has no inputs entry.
	_, soloPresent := svc.components["solo"]
	test.That(t, soloPresent, test.ShouldBeFalse)
	// The kinematic sub is kept even when it is the non-canonical API (gantry under a camera-canonical
	// composite).
	test.That(t, svc.components["noncanon"], test.ShouldEqual, ncGantry)
}

// TestResolveKinematicIdentity checks that ResolveKinematic distinguishes a composite by its kinematic
// subresources' identity, not its kinematic API count. One struct served under several kinematic APIs is
// the same object each time; distinct per-API objects are the unsupported multi-kinematic case.
func TestResolveKinematicIdentity(t *testing.T) {
	logger := logging.NewTestLogger(t)
	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	// One struct served under both kinematic APIs: the same object unwraps once per API.
	shared := &kinematicRes{Named: resource.NewName(armAPI, "shared").AsNamed()}
	sharedWrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "shared"),
		map[resource.API]resource.Resource{armAPI: shared, gantryAPI: shared},
	)
	chosen := ResolveKinematic(sharedWrapper, logger)
	test.That(t, chosen, test.ShouldEqual, shared)

	// Two distinct kinematic objects: the unsupported multi-kinematic device.
	kinArm := &kinematicRes{Named: resource.NewName(armAPI, "twochain").AsNamed()}
	kinGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "twochain").AsNamed()}
	twoChain := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "twochain"),
		map[resource.API]resource.Resource{armAPI: kinArm, gantryAPI: kinGantry},
	)
	test.That(t, ResolveKinematic(twoChain, logger), test.ShouldBeNil)
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

	// A composite whose capabilities come from different subs: the gantry sub implements InputEnabled, the
	// camera sub implements resource.Shaped. FrameContribution collects each from whichever sub provides it.
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
	ie, _, _, refuse = FrameContribution(multiWrapper)
	test.That(t, refuse, test.ShouldBeTrue)
	test.That(t, ie, test.ShouldBeNil)
}
