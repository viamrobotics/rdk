package fake

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
)

const testGripperModelPath = "../../../referenceframe/testfiles/test_gripper.json"

func TestNoModelPath(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name:                "testGripper",
		ConvertedAttributes: &Config{},
	}

	g, err := NewGripper(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	model, err := g.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 0)

	inputs, err := g.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(inputs), test.ShouldEqual, 0)

	err = g.Open(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	_, err = g.Grab(ctx, nil)
	test.That(t, err, test.ShouldBeNil)

	geoms, err := g.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(geoms), test.ShouldEqual, 0)
}

func TestWithModelPath(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	cfg := resource.Config{
		Name: "testGripper",
		ConvertedAttributes: &Config{
			ModelFilePath: testGripperModelPath,
		},
	}

	g, err := NewGripper(ctx, nil, cfg, logger)
	test.That(t, err, test.ShouldBeNil)

	// Model should have 2 DoF (left_joint, right_joint).
	model, err := g.Kinematics(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(model.DoF()), test.ShouldEqual, 2)

	// Initial inputs should be zero-valued (closed).
	inputs, err := g.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(inputs), test.ShouldEqual, 2)
	test.That(t, inputs[0], test.ShouldEqual, 0)
	test.That(t, inputs[1], test.ShouldEqual, 0)

	// Open should set inputs to max (50, 50).
	err = g.Open(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	inputs, err = g.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs[0], test.ShouldEqual, 50.0)
	test.That(t, inputs[1], test.ShouldEqual, 50.0)

	// Grab should set inputs to min (0, 0).
	_, err = g.Grab(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	inputs, err = g.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs[0], test.ShouldEqual, 0.0)
	test.That(t, inputs[1], test.ShouldEqual, 0.0)

	// GoToInputs should set inputs to the given values.
	err = g.GoToInputs(ctx, []referenceframe.Input{25, 30})
	test.That(t, err, test.ShouldBeNil)
	inputs, err = g.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs[0], test.ShouldEqual, 25.0)
	test.That(t, inputs[1], test.ShouldEqual, 30.0)

	// Geometries should return non-empty results.
	geoms, err := g.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(geoms), test.ShouldBeGreaterThan, 0)

	// Geometries should change when inputs change.
	geomsAtPartial := geoms
	err = g.Open(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	geomsAtOpen, err := g.Geometries(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, len(geomsAtOpen), test.ShouldEqual, len(geomsAtPartial))
	// At least one geometry should have a different pose.
	different := false
	for i := range geomsAtOpen {
		if !geomsAtOpen[i].Pose().Point().ApproxEqual(geomsAtPartial[i].Pose().Point()) {
			different = true
			break
		}
	}
	test.That(t, different, test.ShouldBeTrue)
}
