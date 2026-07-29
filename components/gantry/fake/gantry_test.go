package fake

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/golang/geo/r3"
	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
)

func TestDefaultModel(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g, err := NewGantry(ctx, nil, resource.Config{
		Name:                "gantry-1",
		ConvertedAttributes: &Config{},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	lengths, err := g.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{100})

	model, err := g.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.DoF()[0].Max, test.ShouldEqual, 100)

	pose, err := model.Transform([]referenceframe.Input{10})
	test.That(t, err, test.ShouldBeNil)
	// embedded model offsets base by -50mm on X
	test.That(t, spatialmath.R3VectorAlmostEqual(pose.Point(), r3.Vector{X: -40, Y: 0, Z: 0}, 1e-6), test.ShouldBeTrue)
}

func TestLengthMmOverride(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g, err := NewGantry(ctx, nil, resource.Config{
		Name: "gantry-1",
		ConvertedAttributes: &Config{
			LengthMm: 1000,
		},
	}, logger)
	test.That(t, err, test.ShouldBeNil)

	lengths, err := g.Lengths(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, lengths, test.ShouldResemble, []float64{1000})

	model, err := g.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, model.DoF()[0].Max, test.ShouldEqual, 1000)
}

func TestModelPathConflictsWithLength(t *testing.T) {
	conf := &Config{
		ModelFilePath: filepath.Join("..", "..", "..", "referenceframe", "testfiles", "example_gantry.json"),
		LengthMm:      1000,
	}
	_, _, err := conf.Validate("")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "model_path")

	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	_, err = NewGantry(ctx, nil, resource.Config{
		Name:                "gantry-1",
		ConvertedAttributes: conf,
	}, logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "model_path")
}

func TestValidateLengthMm(t *testing.T) {
	_, _, err := (&Config{LengthMm: -1}).Validate("")
	test.That(t, err, test.ShouldNotBeNil)

	_, _, err = (&Config{LengthMm: 500}).Validate("")
	test.That(t, err, test.ShouldBeNil)
}
