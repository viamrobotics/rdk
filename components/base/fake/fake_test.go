package fake

import (
	"context"
	"testing"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/slam/fake"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/test"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/config"
)

func TestFakeBase(t *testing.T) {
	cfg := config.Component{
		Name: "test",
		ConvertedAttributes: &Config{
			BaseModel: "wheeled",
		},
		Frame: &referenceframe.LinkConfig{
			Parent: referenceframe.World,
			Geometry: &spatialmath.GeometryConfig{
				R: 10,
			},
		},
	}

	ctx := context.Background()
	logger := golog.NewTestLogger(t)
	b, err := NewBase(ctx, cfg, logger)
	test.That(t, err, test.ShouldBeNil)
	kb, err := b.(*Base).WrapWithKinematics(ctx, fake.NewSLAM("test", logger))
	test.That(t, err, test.ShouldBeNil)
	expected := referenceframe.FloatsToInputs([]float64{10, 11})
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	inputs, err := kb.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, expected)
}
