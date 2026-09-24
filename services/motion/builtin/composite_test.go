package builtin

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
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
// kinematic sub is kept, and a composite serving two kinematic APIs is refused (not guessed).
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

	// "multi": a composite serving two kinematic APIs (arm + gantry) — unsupported, must be refused.
	multiArm := &kinematicComp{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicComp{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	// "solo": an ordinary component.
	solo := &kinematicComp{Named: resource.NewName(armAPI, "solo").AsNamed()}

	deps := resource.Dependencies{
		resource.NewName(armAPI, "wrap"):     wrapper,
		resource.NewName(camAPI, "wrap"):     wrapper,
		resource.NewName(armAPI, "multi"):    multiArm,
		resource.NewName(gantryAPI, "multi"): multiGantry,
		resource.NewName(armAPI, "solo"):     solo,
	}

	test.That(t, ms.BuiltInReconfigure(ctx, deps, conf), test.ShouldBeNil)

	// The composite wrapper was unwrapped and the kinematic (arm) sub kept — not the wrapper, not the camera.
	test.That(t, ms.components["wrap"], test.ShouldEqual, wrapArm)
	// A multi-kinematic composite is refused, not silently picked.
	_, present := ms.components["multi"]
	test.That(t, present, test.ShouldBeFalse)
	// An ordinary component is unaffected.
	test.That(t, ms.components["solo"], test.ShouldEqual, solo)
}
