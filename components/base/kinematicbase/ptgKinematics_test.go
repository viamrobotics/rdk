// Package kinematicbase contains wrappers that augment bases with information needed for higher level
// control over the base
package kinematicbase

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func TestPTGKinematics(t *testing.T) {
	logger := golog.NewTestLogger(t)

	name, err := resource.NewFromString("is:a:fakebase")
	test.That(t, err, test.ShouldBeNil)

	b := &fake.Base{
		Named:         name.AsNamed(),
		Geometry:      []spatialmath.Geometry{},
		WidthMeters:   0.2,
		TurningRadius: 0.3,
	}

	ctx := context.Background()

	kb, err := WrapWithKinematics(ctx, b, logger, nil, nil, NewKinematicBaseOptions())
	test.That(t, err, test.ShouldBeNil)
	test.That(t, kb, test.ShouldNotBeNil)
	ptgBase, ok := kb.(*ptgBaseKinematics)
	test.That(t, ok, test.ShouldBeTrue)
	test.That(t, ptgBase, test.ShouldNotBeNil)

	dstPIF := referenceframe.NewPoseInFrame(referenceframe.World, spatialmath.NewPoseFromPoint(r3.Vector{X: 999, Y: 0, Z: 0}))

	fs := referenceframe.NewEmptyFrameSystem("test")
	f := kb.Kinematics()
	test.That(t, err, test.ShouldBeNil)
	fs.AddFrame(f, fs.World())
	inputMap := referenceframe.StartPositions(fs)

	plan, err := motionplan.PlanMotion(ctx, motionplan.PlanRequest{
		Logger:      logger,
		Goal:        dstPIF,
		Frame:       f,
		Inputs:      inputMap,
		FrameSystem: fs,
	})
	test.That(t, err, test.ShouldBeNil)
	test.That(t, plan, test.ShouldNotBeNil)
}
