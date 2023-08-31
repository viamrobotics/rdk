package kinematicbase

import (
	"context"
	"math"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
)

func TestNewFakeKinematics(t *testing.T) {
	conf := resource.Config{
		Name: "test",
		Frame: &referenceframe.LinkConfig{
			Parent: referenceframe.World,
			Geometry: &spatialmath.GeometryConfig{
				R: 10,
			},
		},
	}

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	b, err := fakebase.NewBase(ctx, resource.Dependencies{}, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	fakeSLAM := fake.NewSLAM(slam.Named("test"), logger)
	limits, err := fakeSLAM.Limits(ctx)
	test.That(t, err, test.ShouldBeNil)
	limits = append(limits, referenceframe.Limit{-2 * math.Pi, 2 * math.Pi})

	options := NewKinematicBaseOptions()
	options.PositionOnlyMode = false
	kb, err := WrapWithFakeKinematics(ctx, b.(*fakebase.Base), motion.NewSLAMLocalizer(fakeSLAM), limits, options)
	test.That(t, err, test.ShouldBeNil)
	expected := referenceframe.FloatsToInputs([]float64{10, 11, 0})
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	inputs, err := kb.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, expected)
}
