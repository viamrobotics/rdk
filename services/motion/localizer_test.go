package motion_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/golang/geo/r3"
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

func TestNewMovementSensorLocalizer(t *testing.T) {
	nanLat := geo.NewPoint(math.NaN(), 0)
	nanLng := geo.NewPoint(0, math.NaN())
	nanLatLng := geo.NewPoint(math.NaN(), math.NaN())
	nanXR3 := r3.Vector{X: math.NaN(), Y: 0, Z: 0}
	nanOX := spatialmath.OrientationVector{OX: math.NaN()}
	validGP := geo.NewPoint(-70, 40)
	movementSensor := createInjectedMovementSensor("", validGP)
	validP := spatialmath.NewZeroPose()
	// validO := spatialmath.NewOrientationVector()
	type testCase struct {
		description string
		geoPoint    *geo.Point
		pose        spatialmath.Pose
		err         error
	}
	tcs := []testCase{
		{
			description: "NaN lat",
			geoPoint:    nanLat,
			pose:        validP,
			err:         errors.New("lat can't be NaN"),
		},
		{
			description: "NaN lng",
			geoPoint:    nanLng,
			pose:        validP,
			err:         errors.New("lng can't be NaN"),
		},
		{
			description: "NaN lat lng",
			geoPoint:    nanLatLng,
			pose:        validP,
			err:         errors.New("lat can't be NaN"),
		},
		{
			description: "NaN point",
			geoPoint:    validGP,
			pose:        spatialmath.NewPose(nanXR3, nil),
			err:         errors.New("X can't be NaN"),
		},
		{
			description: "NaN orientation",
			geoPoint:    validGP,
			pose:        spatialmath.NewPose(r3.Vector{}, &nanOX),
			err:         errors.New("X can't be NaN"),
		},
	}
	for _, tc := range tcs {
		t.Run(tc.description, func(t *testing.T) {
			localizer, err := motion.NewMovementSensorLocalizer(movementSensor, tc.geoPoint, tc.pose)
			if tc.err != nil {
				test.That(t, err, test.ShouldBeError, tc.err)
				test.That(t, localizer, test.ShouldBeNil)
			} else {
				test.That(t, err, test.ShouldBeNil)
				test.That(t, localizer, test.ShouldNotBeNil)
			}
		})
	}
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
