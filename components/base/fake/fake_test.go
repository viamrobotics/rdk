package fake

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"go.viam.com/test"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/services/slam"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
)

func TestFakeBase(t *testing.T) {
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
	b, err := NewBase(ctx, resource.Dependencies{}, conf, logger)
	test.That(t, err, test.ShouldBeNil)
	fakeSLAM := fake.NewSLAM(slam.Named("test"), logger)
	limits, err := fakeSLAM.GetLimits(ctx)
	test.That(t, err, test.ShouldBeNil)

	localizer, err := motion.NewLocalizer(ctx, fakeSLAM)
	test.That(t, err, test.ShouldBeNil)

	kb, err := b.(*Base).WrapWithKinematics(ctx, localizer, limits)
	test.That(t, err, test.ShouldBeNil)
	expected := referenceframe.FloatsToInputs([]float64{10, 11, 0})
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	inputs, err := kb.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, expected)
}
