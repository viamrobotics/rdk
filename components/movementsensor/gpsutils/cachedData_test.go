package gpsutils

import (
	"context"
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
