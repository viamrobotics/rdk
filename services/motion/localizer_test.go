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

func createInjectedMovementSensor(gpsPoint *geo.Point) *inject.MovementSensor {
	injectedMovementSensor := inject.NewMovementSensor("")
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
	movementSensor := createInjectedMovementSensor(validGP)
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
			description: "valid",
			geoPoint:    validGP,
			pose:        validP,
			err:         nil,
		},
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

func TestCurrentPosition(t *testing.T) {
	ctx := context.Background()
	origin := geo.NewPoint(-70, 40)

	t.Run("returns an error if the position returned an error", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		errExpected := errors.New("no position for you")
		movementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			return nil, 0, errExpected
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns an error if the position is invalid", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		errExpected := errors.New("lat can't be NaN")
		movementSensor.PositionFunc = func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
			return geo.NewPoint(math.NaN(), 0), 0, nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("returns an error if the properties returned an error", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		errExpected := errors.New("no properties for you")
		movementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return nil, errExpected
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when heading is supported returns an error if the heading returned an error", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		p, err := movementSensor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.CompassHeadingSupported, test.ShouldBeTrue)
		errExpected := errors.New("no heading for you")
		movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 0, errExpected
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when heading is supported returns an error if the heading is invalid", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		p, err := movementSensor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.CompassHeadingSupported, test.ShouldBeTrue)
		errExpected := errors.New("heading can't be NaN")
		movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return math.NaN(), nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when heading is NOT supported but orientation is, returns an error if the orientation returns an error", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		p, err := movementSensor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.CompassHeadingSupported, test.ShouldBeTrue)
		errExpected := errors.New("no orientation for you")
		movementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{CompassHeadingSupported: false, OrientationSupported: true}, nil
		}
		movementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return nil, errExpected
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when heading is NOT supported but orientation is, returns an error if the orientation is invalid", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		p, err := movementSensor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.CompassHeadingSupported, test.ShouldBeTrue)
		errExpected := errors.New("X can't be NaN")
		movementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{CompassHeadingSupported: false, OrientationSupported: true}, nil
		}
		movementSensor.OrientationFunc = func(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
			return &spatialmath.Quaternion{Imag: math.NaN()}, nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when neither heading nor orientation are supported returns an error", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		p, err := movementSensor.Properties(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, p.CompassHeadingSupported, test.ShouldBeTrue)
		errExpected := errors.New("could not get orientation from Localizer")
		movementSensor.PropertiesFunc = func(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
			return &movementsensor.Properties{}, nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		_, err = localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeError, errExpected)
	})

	t.Run("when the movement sensor says that the robot is facing north, reports orientation as pointing north", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		heading, err := movementSensor.CompassHeading(ctx, nil)
		test.That(t, err, test.ShouldBeNil)
		test.That(t, heading, test.ShouldEqual, 0)

		pif, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		// A heading of 0 means we are pointing north.
		test.That(t, spatialmath.OrientationAlmostEqual(
			pif.Pose().Orientation(),
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 0}),
			test.ShouldBeTrue,
		)
	})

	t.Run("when the movement sensor says that the robot is facing north west, reports orientation as pointing north west", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		// Update heading to point northwest
		movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 315, nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		pif, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)
		// A heading of 315 means we are pointing northwest.
		test.That(t, spatialmath.OrientationAlmostEqual(
			pif.Pose().Orientation(),
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: 45}),
			test.ShouldBeTrue,
		)
	})

	t.Run("when the movement sensor says that the robot is facing east, reports orientation as pointing north east", func(t *testing.T) {
		movementSensor := createInjectedMovementSensor(origin)
		// heading to point east
		movementSensor.CompassHeadingFunc = func(ctx context.Context, extra map[string]interface{}) (float64, error) {
			return 90, nil
		}
		localizer, err := motion.NewMovementSensorLocalizer(movementSensor, origin, spatialmath.NewZeroPose())
		test.That(t, err, test.ShouldBeNil)

		pif, err := localizer.CurrentPosition(ctx)
		test.That(t, err, test.ShouldBeNil)

		// A heading of 90 means we are pointing east.
		test.That(t, spatialmath.OrientationAlmostEqual(
			pif.Pose().Orientation(),
			&spatialmath.OrientationVectorDegrees{OZ: 1, Theta: -90}),
			test.ShouldBeTrue,
		)
	})
}
