package gpsrtkserial

import (
	"context"
	"errors"
	"math"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

type mockDataReader struct{}

func (d *mockDataReader) Messages() chan string {
	return nil
}

func (d *mockDataReader) Close() error {
	return nil
}

func TestValidateRTK(t *testing.T) {
	path := "path"
	t.Run("valid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           path,
			SerialBaudRate:       115200,
		}
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           path,
			SerialBaudRate:       115200,
		}

		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "ntrip_url"))
	})

	t.Run("invalid config", func(t *testing.T) {
		cfg := Config{
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			SerialPath:           "",
			SerialBaudRate:       115200,
		}

		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "serial_path"))
	})
}

func TestReconfigure(t *testing.T) {
	g := &rtkSerial{
		writePath: "/dev/ttyUSB0",
		wbaud:     9600,
		logger:    logging.NewTestLogger(t),
	}

	conf := resource.Config{
		Name: "reconfig1",
		ConvertedAttributes: &Config{
			SerialPath:           "/dev/ttyUSB1",
			SerialBaudRate:       115200,
			NtripURL:             "http//fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
		},
	}

	err := g.Reconfigure(context.Background(), nil, conf)

	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.writePath, test.ShouldResemble, "/dev/ttyUSB1")
	test.That(t, g.wbaud, test.ShouldEqual, 115200)
}

func TestPosition(t *testing.T) {
	testLastPosition := geo.NewPoint(42.1, 123)
	// WITH LAST ERROR

	// If there is last error and no last position, return NaN
	t.Run("position with last error and no last position", func(t *testing.T) {
		g := &rtkSerial{
			err: movementsensor.NewLastError(1, 1),
		}
		g.err.Set(errors.New("last error test"))

		pos, alt, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.IsPositionNaN(pos), test.ShouldBeTrue)
		test.That(t, math.IsNaN(alt), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeError, "last error test")
	})

	// If there is last error and last position, return last position
	t.Run("position with last error and last position", func(t *testing.T) {
		g := &rtkSerial{
			err:          movementsensor.NewLastError(1, 1),
			lastposition: movementsensor.NewLastPosition(),
		}
		g.lastposition.SetLastPosition(testLastPosition)

		g.err.Set(errors.New("last position"))
		expectedPos := geo.NewPoint(42.1, 123.)

		pos, alt, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPos), test.ShouldBeTrue)
		test.That(t, alt, test.ShouldEqual, 0.0)
		test.That(t, err, test.ShouldBeNil)
	})

	// NO LAST ERROR, but with cachedData ERROR

	// If there is no last error, invalid current position and no last position, return NaN
	t.Run("invalid position with invalid last position, with position error", func(t *testing.T) {
		g := &rtkSerial{
			err:        movementsensor.NewLastError(1, 1),
			cachedData: gpsutils.NewCachedData(&mockDataReader{}, logging.NewTestLogger(t)),
		}

		pos, alt, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.IsPositionNaN(pos), test.ShouldBeTrue)
		test.That(t, math.IsNaN(alt), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeError, "nil gps location, check nmea message parsing")
	})

	// If there is no last error, invalid current position and valid last position, return last position
	t.Run("invalid position with valid last position, with position error", func(t *testing.T) {
		g := &rtkSerial{
			err:          movementsensor.NewLastError(1, 1),
			lastposition: movementsensor.NewLastPosition(),
			cachedData:   gpsutils.NewCachedData(&mockDataReader{}, logging.NewTestLogger(t)),
		}

		g.lastposition.SetLastPosition(testLastPosition)
		expectedPos := geo.NewPoint(42.1, 123.)

		pos, _, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPos), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	// NO ERRORS

	// Invalid current position from NMEA message, return last known position
	t.Run("invalid position with valid last position, no error", func(t *testing.T) {
		g := &rtkSerial{
			err:          movementsensor.NewLastError(1, 1),
			lastposition: movementsensor.NewLastPosition(),
			cachedData:   gpsutils.NewCachedData(&mockDataReader{}, logging.NewTestLogger(t)),
		}

		// NMEA sentence with invalid position, Fix quality is 0
		nmeaSentenceInvalid := "$GPGGA,172814.0,123.123,N,234.234,W,0,6,1.2,18.893,M,-25.669,M,2.0,0031*4F"
		g.cachedData.ParseAndUpdate(nmeaSentenceInvalid)
		g.lastposition.SetLastPosition(testLastPosition)
		expectedPos := geo.NewPoint(42.1, 123.) // Last known position

		pos, _, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.ArePointsEqual(pos, expectedPos), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeNil)
	})

	// Valid current position, should return current position
	t.Run("valid position, no error", func(t *testing.T) {
		g := &rtkSerial{
			err:          movementsensor.NewLastError(1, 1),
			lastposition: movementsensor.NewLastPosition(),
			cachedData:   gpsutils.NewCachedData(&mockDataReader{}, logging.NewTestLogger(t)),
		}

		// Valid NMEA sentence
		nmeaSentenceValid := "$GPGGA,172814.0,3723.46587704,N,12202.26957864,W,2,6,1.2,18.893,M,-25.669,M,2.0,0031*4F"
		g.cachedData.ParseAndUpdate(nmeaSentenceValid)
		g.lastposition.SetLastPosition(testLastPosition)

		pos, _, err := g.Position(context.Background(), nil)

		expectedLat := (37 + 23.46587704/60)
		expectedLng := -(122 + 2.26957864/60)

		test.That(t, pos.Lat(), test.ShouldAlmostEqual, expectedLat)
		test.That(t, pos.Lng(), test.ShouldAlmostEqual, expectedLng)
		test.That(t, err, test.ShouldBeNil)
	})
}
