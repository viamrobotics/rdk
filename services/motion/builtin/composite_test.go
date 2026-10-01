package builtin

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/testutils/inject"
)

// kinematicComp implements framesystem.InputEnabled (a kinematic component); plainComp does not.
type kinematicComp struct {
	resource.Named
	resource.TriviallyCloseable
}

func (kinematicComp) Kinematics(context.Context) (referenceframe.Model, error) { return nil, nil }

func (kinematicComp) CurrentInputs(context.Context) ([]referenceframe.Input, error) { return nil, nil }
func (kinematicComp) GoToInputs(context.Context, ...[]referenceframe.Input) error   { return nil }

type plainComp struct {
	resource.Named
	resource.TriviallyCloseable
}

// TestBuiltInReconfigureComposite verifies motion unwraps a composite dependency to the correct
// per-API sub before classifying it, and applies the one-kinematic-API rule to the component map:
// a composite wrapper aliased under several API names resolves to each API's real sub, the single
// kinematic sub is kept (even when its API is not canonical), a service-typed sub routes to its own
// map rather than the component map, and a composite serving two kinematic APIs is refused (not guessed).
func TestBuiltInReconfigureComposite(t *testing.T) {
	ctx := context.Background()
	ms := &builtIn{logger: logging.NewTestLogger(t)}
	conf := resource.Config{ConvertedAttributes: &Config{}}

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// A modular composite "wrap" serving arm (kinematic) + camera (non-kinematic): in deps it is the
	// MultiAPIResource wrapper, aliased under both API names.
	wrapArm := &kinematicComp{Named: resource.NewName(armAPI, "wrap").AsNamed()}
	wrapCam := &plainComp{Named: resource.NewName(camAPI, "wrap").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "wrap"),
		[]resource.API{armAPI, camAPI},
		map[resource.API]resource.Resource{armAPI: wrapArm, camAPI: wrapCam},
	)

	// "noncanon": a composite whose kinematic API is NOT canonical — camera (canonical, non-kinematic)
	// + gantry (kinematic). The kinematic sub must be kept regardless of which API sorts first, so this
	// distinguishes keeping the kinematic sub from keeping whichever sub happens to be first.
	ncCam := &plainComp{Named: resource.NewName(camAPI, "noncanon").AsNamed()}
	ncGantry := &kinematicComp{Named: resource.NewName(gantryAPI, "noncanon").AsNamed()}
	ncWrapper := resource.NewMultiAPIResource(
		resource.NewName(camAPI, "noncanon"),
		[]resource.API{camAPI, gantryAPI},
		map[resource.API]resource.Resource{camAPI: ncCam, gantryAPI: ncGantry},
	)

	// "svccombo": a composite serving arm (component) + vision (service). The service-typed sub must be
	// routed to visionServices, leaving only the arm sub in the component map under the shared name.
	svcArm := &kinematicComp{Named: resource.NewName(armAPI, "svccombo").AsNamed()}
	svcVision := inject.NewVisionService("svccombo")
	svcWrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "svccombo"),
		[]resource.API{armAPI, vision.API},
		map[resource.API]resource.Resource{armAPI: svcArm, vision.API: svcVision},
	)

	// "multi": a composite serving two kinematic APIs (arm + gantry) — unsupported, must be refused.
	multiArm := &kinematicComp{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicComp{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	// "solo": an ordinary component.
	solo := &kinematicComp{Named: resource.NewName(armAPI, "solo").AsNamed()}

	deps := resource.Dependencies{
		resource.NewName(armAPI, "wrap"):         wrapper,
		resource.NewName(camAPI, "wrap"):         wrapper,
		resource.NewName(camAPI, "noncanon"):     ncWrapper,
		resource.NewName(gantryAPI, "noncanon"):  ncWrapper,
		resource.NewName(armAPI, "svccombo"):     svcWrapper,
		resource.NewName(vision.API, "svccombo"): svcWrapper,
		resource.NewName(armAPI, "multi"):        multiArm,
		resource.NewName(gantryAPI, "multi"):     multiGantry,
		resource.NewName(armAPI, "solo"):         solo,
	}

	test.That(t, ms.BuiltInReconfigure(ctx, deps, conf), test.ShouldBeNil)

	// The composite wrapper was unwrapped and the kinematic (arm) sub kept — not the wrapper, not the camera.
	test.That(t, ms.components["wrap"], test.ShouldEqual, wrapArm)
	// The kinematic sub is kept even when it is the non-canonical API (gantry under a camera-canonical
	// composite), not just when it sorts first.
	test.That(t, ms.components["noncanon"], test.ShouldEqual, ncGantry)
	// The vision sub routed to visionServices; only the arm sub remains in the component map.
	test.That(t, ms.visionServices["svccombo"], test.ShouldEqual, svcVision)
	test.That(t, ms.components["svccombo"], test.ShouldEqual, svcArm)
	// A multi-kinematic composite is refused, not silently picked.
	_, present := ms.components["multi"]
	test.That(t, present, test.ShouldBeFalse)
	// An ordinary component is unaffected.
	test.That(t, ms.components["solo"], test.ShouldEqual, solo)
}

// TestBuiltInReconfigureSingleStructMultiKinematic verifies motion keeps a single struct served under
// two kinematic APIs as one chain in the component map -- consistent with the frame system and
// KinematicClassify -- rather than refusing it as it does a composite backed by two DISTINCT kinematic subs.
func TestBuiltInReconfigureSingleStructMultiKinematic(t *testing.T) {
	ctx := context.Background()
	ms := &builtIn{logger: logging.NewTestLogger(t)}
	conf := resource.Config{ConvertedAttributes: &Config{}}

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")

	shared := &kinematicComp{Named: resource.NewName(armAPI, "combo").AsNamed()}
	wrapper := resource.NewMultiAPIResource(
		resource.NewName(armAPI, "combo"),
		[]resource.API{armAPI, gantryAPI},
		map[resource.API]resource.Resource{armAPI: shared, gantryAPI: shared},
	)
	deps := resource.Dependencies{
		resource.NewName(armAPI, "combo"):    wrapper,
		resource.NewName(gantryAPI, "combo"): wrapper,
	}

	test.That(t, ms.BuiltInReconfigure(ctx, deps, conf), test.ShouldBeNil)
	// One shared kinematic object under two APIs is one chain, kept in the component map (not refused).
	test.That(t, ms.components["combo"], test.ShouldEqual, shared)
}
