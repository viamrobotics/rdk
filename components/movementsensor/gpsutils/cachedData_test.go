package gpsutils

import (
	"context"
	"errors"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
)

var loc = geo.NewPoint(90, 1)

const (
	alt        = 50.5
	speed      = 5.4
	activeSats = 1
	totalSats  = 2
	hAcc       = 0.7
	vAcc       = 0.8
	fix        = 1
)

type mockDataReader struct{}

func (d *mockDataReader) Messages() chan string {
	return nil
}

func (d *mockDataReader) Close() error {
	return nil
}

func TestReadingsSerial(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)
	g.nmeaData = NmeaParser{
		Location:   loc,
		Alt:        alt,
		Speed:      speed,
		VDOP:       vAcc,
		HDOP:       hAcc,
		SatsInView: totalSats,
		SatsInUse:  activeSats,
		FixQuality: fix,
	}

	loc1, alt1, err := g.Position(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, loc1, test.ShouldEqual, loc)
	test.That(t, alt1, test.ShouldEqual, alt)

	speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeNil)
	test.That(t, speed1.Y, test.ShouldEqual, speed)

	readings := g.GetCommonReadings(ctx)
	test.That(t, readings.FixValue, test.ShouldEqual, fix)
	test.That(t, readings.SatsInUse, test.ShouldEqual, activeSats)
	test.That(t, readings.SatsInView, test.ShouldEqual, totalSats)
}

func TestPosition(t *testing.T) {
	testLastPosition := geo.NewPoint(32.4, 54.2)
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)

	t.Run("no current location", func(t *testing.T) {
		g.lastPosition.SetLastPosition(testLastPosition)
		g.nmeaData = NmeaParser{
			Location: nil,
		}

		expectedPoint := geo.NewPoint(32.4, 54.2)

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeError, "nil gps location, check nmea message parsing")
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPoint), test.ShouldBeTrue)
		test.That(t, alt, test.ShouldEqual, 0.0)
	})

	t.Run("current location zero so return last known", func(t *testing.T) {
		g.lastPosition.SetLastPosition(testLastPosition)
		g.nmeaData = NmeaParser{
			Location: geo.NewPoint(0, 0),
			Alt:      12.1,
		}

		expectedPoint := geo.NewPoint(32.4, 54.2)

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPoint), test.ShouldBeTrue)
		test.That(t, alt, test.ShouldEqual, 12.1)

		// Check that the last known position was not updated
		test.That(t, movementsensor.ArePointsEqual(g.lastPosition.GetLastPosition(), expectedPoint), test.ShouldBeTrue)
	})

	t.Run("Valid current location", func(t *testing.T) {
		newPoint := geo.NewPoint(1.1, 1.2)
		g.lastPosition.SetLastPosition(testLastPosition)
		g.nmeaData = NmeaParser{
			Location: newPoint,
			Alt:      1.3,
		}

		expectedPoint := geo.NewPoint(1.1, 1.2)

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPoint), test.ShouldBeTrue)
		test.That(t, alt, test.ShouldEqual, 1.3)

		// Check that the last known position was updated
		test.That(t, movementsensor.ArePointsEqual(g.lastPosition.GetLastPosition(), expectedPoint), test.ShouldBeTrue)
	})
}

func TestLinearVelocity(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)

	t.Run("no compass heading", func(t *testing.T) {
		g.nmeaData = NmeaParser{
			CompassHeading: math.NaN(),
		}

		speed, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed.X, test.ShouldEqual, 0.0)
		test.That(t, speed.Y, test.ShouldEqual, 0.0)
	})

	t.Run("test sample velocity", func(t *testing.T) {
		// Test all compass quadrants to ensure the correct velocity is returned
		// Ensures correct angle and orientation of velocity vector

		// All angles are compass headings

		// Q1: 60 degrees
		g.nmeaData = NmeaParser{
			Speed:          speed,
			CompassHeading: 60.,
		}

		expectedX := speed * math.Sqrt(3) / 2
		expectedY := speed / 2

		speed1, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed1.X, test.ShouldAlmostEqual, expectedX)
		test.That(t, speed1.Y, test.ShouldAlmostEqual, expectedY)

		// Q2: 150 degrees
		g.nmeaData.CompassHeading = 150.

		expectedX = speed / 2.
		expectedY = speed * -math.Sqrt(3) / 2.

		speed2, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed2.X, test.ShouldAlmostEqual, expectedX)
		test.That(t, speed2.Y, test.ShouldAlmostEqual, expectedY)

		// Q3: 225 degrees
		g.nmeaData.CompassHeading = 225.

		expectedX = speed * -math.Sqrt(2) / 2.
		expectedY = speed * -math.Sqrt(2) / 2.

		speed3, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed3.X, test.ShouldAlmostEqual, expectedX)
		test.That(t, speed3.Y, test.ShouldAlmostEqual, expectedY)

		// Q4: 310 degrees
		g.nmeaData.CompassHeading = 310.

		// Convert 310 degrees compass to standard Cartesian coordinates
		expectedX = speed * math.Cos(140.*(math.Pi/180))
		expectedY = speed * math.Sin(140.*(math.Pi/180))

		speed4, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed4.X, test.ShouldAlmostEqual, expectedX)
		test.That(t, speed4.Y, test.ShouldAlmostEqual, expectedY)
	})
}

func TestAccuracy(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)
	g.err.Set(errors.New("test error"))

	g.nmeaData = NmeaParser{
		HDOP:           hAcc,
		VDOP:           vAcc,
		FixQuality:     fix,
		CompassHeading: 90.,
	}

	acc, err := g.Accuracy(ctx, make(map[string]interface{}))
	test.That(t, err, test.ShouldBeError, "test error")
	test.That(t, acc.Hdop, test.ShouldEqual, hAcc)
	test.That(t, acc.Vdop, test.ShouldEqual, vAcc)
	test.That(t, acc.NmeaFix, test.ShouldEqual, fix)

	acMap := acc.AccuracyMap
	test.That(t, acMap["hDOP"], test.ShouldEqual, hAcc)
	test.That(t, acMap["vDOP"], test.ShouldEqual, vAcc)
}

func TestCompassHeading(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)

	t.Run("no valid compass heading, so return last known", func(t *testing.T) {
		lastCompassHeading := 90.
		g.lastCompassHeading.SetLastCompassHeading(lastCompassHeading)
		g.nmeaData = NmeaParser{
			CompassHeading: math.NaN(),
		}

		heading, err := g.CompassHeading(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, heading, test.ShouldEqual, lastCompassHeading)

		// Check that the last known compass heading was not updated
		test.That(t, g.lastCompassHeading.GetLastCompassHeading(), test.ShouldEqual, lastCompassHeading)
	})

	t.Run("valid current compass heading", func(t *testing.T) {
		lastCompassHeading := 90.
		newCompassHeading := 180.
		g.lastCompassHeading.SetLastCompassHeading(lastCompassHeading)
		g.nmeaData = NmeaParser{
			CompassHeading: newCompassHeading,
		}

		heading, err := g.CompassHeading(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, heading, test.ShouldEqual, 180.)

		// Check that the last known compass heading was updated
		test.That(t, g.lastCompassHeading.GetLastCompassHeading(), test.ShouldEqual, newCompassHeading)
	})
}

func TestCompassDegreeError(t *testing.T) {
	p2 := geo.NewPoint(1, 1)

	g := NewCachedData(&mockDataReader{}, logging.NewTestLogger(t))
	cde := g.calculateCompassDegreeError(nil, p2)
	test.That(t, math.IsNaN(cde), test.ShouldBeTrue)
}
