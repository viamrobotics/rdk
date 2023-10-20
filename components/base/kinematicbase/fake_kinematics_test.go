package kinematicbase

import (
	"context"
	"testing"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	fakebase "go.viam.com/rdk/components/base/fake"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func TestNewFakeDiffDriveKinematics(t *testing.T) {
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
	ms := inject.NewMovementSensor("test")
	ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(0, 0), 0, nil
	}
	ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}
	localizer := motion.NewMovementSensorLocalizer(ms, geo.NewPoint(0, 0), spatialmath.NewZeroPose())
	limits := []referenceframe.Limit{{Min: -100, Max: 100}, {Min: -100, Max: 100}}

	options := NewKinematicBaseOptions()
	options.PositionOnlyMode = false
	noise := spatialmath.NewPoseFromPoint(r3.Vector{1, 0, 0})
	kb, err := WrapWithFakeDiffDriveKinematics(ctx, b.(*fakebase.Base), localizer, limits, options, noise)
	test.That(t, err, test.ShouldBeNil)
	expected := referenceframe.FloatsToInputs([]float64{10, 11})
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	inputs, err := kb.CurrentInputs(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, inputs, test.ShouldResemble, expected)
	pose, err := kb.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincident(pose.Pose(), spatialmath.NewPoseFromPoint(r3.Vector{11, 11, 0})), test.ShouldBeTrue)
}

func TestNewFakePTGKinematics(t *testing.T) {
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
	ms := inject.NewMovementSensor("test")
	ms.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(0, 0), 0, nil
	}
	ms.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	ms.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	options := NewKinematicBaseOptions()
	options.PositionOnlyMode = false
	noise := spatialmath.NewPoseFromPoint(r3.Vector{1, 0, 0})
	kb, err := WrapWithFakePTGKinematics(ctx, b.(*fakebase.Base), logger, referenceframe.World, options, spatialmath.NewZeroPose(), noise)
	test.That(t, err, test.ShouldBeNil)

	startpose, err := kb.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	realStartPose := spatialmath.PoseBetweenInverse(noise, startpose.Pose())

	expected := referenceframe.FloatsToInputs([]float64{0, 1.23, 110})
	expectedPose, err := kb.Kinematics().Transform(expected)
	test.That(t, err, test.ShouldBeNil)
	expectedPose = spatialmath.Compose(spatialmath.Compose(realStartPose, expectedPose), noise)
	test.That(t, kb.GoToInputs(ctx, expected), test.ShouldBeNil)
	pose, err := kb.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, spatialmath.PoseAlmostCoincident(pose.Pose(), expectedPose), test.ShouldBeTrue)
}
