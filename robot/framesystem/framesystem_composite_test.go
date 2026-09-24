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
// (which share a short name — e.g. a remote composite surfaced as one sub-client per co-equal API)
// into its short-name-keyed component map: it keeps the single kinematic sub, refuses a composite that
// serves more than one kinematic API (unsupported) instead of crashing, and never errors on the
// duplicate short name.
func TestBuiltInReconfigureComposite(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)

	svcIface, err := New(ctx, resource.Dependencies{}, logger)
	test.That(t, err, test.ShouldBeNil)
	svc := svcIface.(*frameSystemService)

	armAPI := resource.APINamespaceRDK.WithComponentType("arm")
	gantryAPI := resource.APINamespaceRDK.WithComponentType("gantry")
	camAPI := resource.APINamespaceRDK.WithComponentType("camera")

	// "combo": a composite serving one kinematic API (arm) + one non-kinematic (camera).
	comboKin := &kinematicRes{Named: resource.NewName(armAPI, "combo").AsNamed()}
	comboCam := &plainRes{Named: resource.NewName(camAPI, "combo").AsNamed()}
	// "multi": a composite serving two kinematic APIs (arm + gantry) — unsupported, must be refused.
	multiArm := &kinematicRes{Named: resource.NewName(armAPI, "multi").AsNamed()}
	multiGantry := &kinematicRes{Named: resource.NewName(gantryAPI, "multi").AsNamed()}
	// "solo": an ordinary resource.
	solo := &plainRes{Named: resource.NewName(camAPI, "solo").AsNamed()}

	deps := resource.Dependencies{
		resource.NewName(armAPI, "combo"):    comboKin,
		resource.NewName(camAPI, "combo"):    comboCam,
		resource.NewName(armAPI, "multi"):    multiArm,
		resource.NewName(gantryAPI, "multi"): multiGantry,
		resource.NewName(camAPI, "solo"):     solo,
	}

	// No error on the duplicate short names (the old code returned DuplicateResourceNameError).
	err = svc.BuiltInReconfigure(ctx, deps, resource.Config{ConvertedAttributes: &Config{}})
	test.That(t, err, test.ShouldBeNil)

	// The composite keeps its one kinematic sub (so CurrentInputs lands on the real kinematic resource).
	test.That(t, svc.components["combo"], test.ShouldEqual, comboKin)
	// A multi-kinematic composite is refused, not silently picked.
	_, present := svc.components["multi"]
	test.That(t, present, test.ShouldBeFalse)
	// An ordinary resource is unaffected.
	test.That(t, svc.components["solo"], test.ShouldEqual, solo)
}
