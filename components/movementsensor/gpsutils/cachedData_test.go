package gpsutils

import (
	"context"
	"errors"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

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

	fix1, err := g.ReadFix(ctx)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, fix1, test.ShouldEqual, fix)
}

func TestPosition(t *testing.T) {
	ctx := context.Background()
	logger := logging.NewTestLogger(t)
	g := NewCachedData(&mockDataReader{}, logger)

	t.Run("no current location", func(t *testing.T) {
		g.lastPosition.SetLastPosition(geo.NewPoint(32.4, 54.2))
		g.nmeaData = NmeaParser{
			Location: nil,
		}

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldNotBeNil)
		test.That(t, pos.Lat(), test.ShouldEqual, 32.4)
		test.That(t, pos.Lng(), test.ShouldEqual, 54.2)
		test.That(t, alt, test.ShouldEqual, 0.0)
	})

	t.Run("current location zero so return last known", func(t *testing.T) {
		g.lastPosition.SetLastPosition(geo.NewPoint(32.4, 54.2))
		g.nmeaData = NmeaParser{
			Location: geo.NewPoint(0, 0),
			Alt:      12.1,
		}

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.Lat(), test.ShouldEqual, 32.4)
		test.That(t, pos.Lng(), test.ShouldEqual, 54.2)
		test.That(t, alt, test.ShouldEqual, 12.1)

		// Check that the last known position was not updated
		test.That(t, g.lastPosition.GetLastPosition().Lat(), test.ShouldEqual, 32.4)
		test.That(t, g.lastPosition.GetLastPosition().Lng(), test.ShouldEqual, 54.2)
	})

	t.Run("Valid current location", func(t *testing.T) {
		g.lastPosition.SetLastPosition(geo.NewPoint(32.4, 54.2))
		g.nmeaData = NmeaParser{
			Location: geo.NewPoint(1.1, 1.2),
			Alt:      1.3,
		}

		pos, alt, err := g.Position(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, pos.Lat(), test.ShouldEqual, 1.1)
		test.That(t, pos.Lng(), test.ShouldEqual, 1.2)
		test.That(t, alt, test.ShouldEqual, 1.3)

		// Check that the last known position was updated
		test.That(t, g.lastPosition.GetLastPosition().Lat(), test.ShouldEqual, 1.1)
		test.That(t, g.lastPosition.GetLastPosition().Lng(), test.ShouldEqual, 1.2)
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
		// Two quandrants of the compass
		// Enough to verify quadrant signs are correct

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

		g.nmeaData.CompassHeading = 150.

		expectedX = speed / 2.
		expectedY = speed * -math.Sqrt(3) / 2.

		speed2, err := g.LinearVelocity(ctx, make(map[string]interface{}))
		test.That(t, err, test.ShouldBeNil)
		test.That(t, speed2.X, test.ShouldAlmostEqual, expectedX)
		test.That(t, speed2.Y, test.ShouldAlmostEqual, expectedY)
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
