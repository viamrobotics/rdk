package motion_test

import (
	"context"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/services/motion"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/testutils/inject"
)

func createInjectedCompassMovementSensor(name string, gpsPoint *geo.Point) *inject.MovementSensor {
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

func createInjectedOrientationMovementSensor(orient spatialmath.Orientation) *inject.MovementSensor {
	injectedMovementSensor := inject.NewMovementSensor("")
	injectedMovementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
		return geo.NewPoint(0, 0), 0, nil
	}
	injectedMovementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
		return orient, nil
	}
	injectedMovementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
		return &movementsensor.Properties{OrientationSupported: true}, nil
	}

	return injectedMovementSensor
}

func TestLocalizerOrientation(t *testing.T) {
	ctx := context.Background()

	origin := geo.NewPoint(-70, 40)
	movementSensor := createInjectedCompassMovementSensor("", origin)
	localizer := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())

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

func TestCorrectStartPose(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	origin := geo.NewPoint(0, 0)
	t.Run("Test angle from +Y to +X, +Y quadrant", func(t *testing.T) {
		t.Parallel()
		// -45
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 1, OY: 1, OZ: 1}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -45.)
	})
	t.Run("Test angle from +Y to -X, +Y quadrant", func(t *testing.T) {
		t.Parallel()
		// 45
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: -1, OY: 1, OZ: 1}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 45.)
	})
	t.Run("Test angle from +Y to +X, -Y quadrant", func(t *testing.T) {
		t.Parallel()
		// -135
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 1, OY: -1, OZ: 1}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -135.)
	})
	t.Run("Test angle from +Y to -X, -Y quadrant", func(t *testing.T) {
		t.Parallel()
		// 135
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: -1, OY: -1, OZ: 1}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 135.)
	})
	t.Run("Test non-multiple-of-45 angle from +Y to +X, +Y quadrant", func(t *testing.T) {
		t.Parallel()
		// -30
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 1, OY: math.Sqrt(3), OZ: 1}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, -30.)
	})
	t.Run("Test orientation already at OZ=1", func(t *testing.T) {
		t.Parallel()
		// 127
		askewOrient := &spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 127.}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		corrected, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, corrected.Pose().Orientation().OrientationVectorDegrees().Theta, test.ShouldAlmostEqual, 127.)
	})
	t.Run("Test pointing-straight-up error", func(t *testing.T) {
		t.Parallel()
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 0, OY: 1, OZ: 0, Theta: 90}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		_, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "orientation appears to be pointing straight up, cannot project to 2d")
	})
	t.Run("Test pointing-straight-down error", func(t *testing.T) {
		t.Parallel()
		askewOrient := &spatialmath.OrientationVectorDegrees{OX: 0, OY: 1, OZ: 0, Theta: -90}
		movementSensor := createInjectedOrientationMovementSensor(askewOrient)
		localizer := motion.TwoDLocalizer(motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose()))
		_, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, err.Error(), test.ShouldEqual, "orientation appears to be pointing straight down, cannot project to 2d")
	})
}
