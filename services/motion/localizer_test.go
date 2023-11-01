package motion_test

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func createInjectedMovementSensor(name string, gpsPoint *geo.Point) *inject.MovementSensor {
	injectedMovementSensor := inject.NewMovementSensor(name)
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return gpsPoint, 0, nil
	}
	injectedMovementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 0, nil
	}
	injectedMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{CompassHeadingSupported: true}, nil
	}

	return injectedMovementSensor
}

func TestLocalizerOrientation(t *testing.T) {
	ctx := context.Background()

	origin := geo.NewPoint(-70, 40)
	movementSensor := createInjectedMovementSensor("", origin)
	localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
	test.That(t, err, test.ShouldBeNil)

	heading, err := movementSensor.CompassHeading(ctx, nil)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, heading, test.ShouldEqual, 0)

	pif, err := localizer.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)

	// A compass heading of 0 means we are pointing north.
	test.That(t, spatialmath.OrientationAlmostEqual(
		pif.Pose().Orientation(),
		&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0}),
		test.ShouldBeTrue,
	)

	// Update compass heading to point northwest
	movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 315, nil
	}

	pif, err = localizer.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)
	// A compass heading of 315 means we are pointing northwest.
	test.That(t, spatialmath.OrientationAlmostEqual(
		pif.Pose().Orientation(),
		&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45}),
		test.ShouldBeTrue,
	)

	// Update compass heading to point east
	movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
		return 90, nil
	}
	pif, err = localizer.CurrentPosition(ctx)
	test.That(t, err, test.ShouldBeNil)

	// A compass heading of 90 means we are pointing east.
	test.That(t, spatialmath.OrientationAlmostEqual(
		pif.Pose().Orientation(),
		&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90}),
		test.ShouldBeTrue,
	)
}
