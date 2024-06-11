package gpsrtkserial

import (
	"context"
	"errors"
	"math"
	"testing"

	"go.viam.com/test"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

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

	t.Run("position with last error and no last position", func(t *testing.T) {
		g := &rtkSerial{
			err: movementsensor.NewLastError(1, 1),
		}
		g.err.Set(errors.New("last error test"))

		pos, alt, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.IsPositionNaN(point), test.ShouldBeTrue)
		test.That(t, math.IsNaN(flt), test.ShouldBeTrue)
		test.That(t, err, test.ShouldBeError, "last error test")
	})

	t.Run("position with last error and last position", func(t *testing.T) {
		g := &rtkSerial{
			err:          movementsensor.NewLastError(1, 1),
			lastposition: movementsensor.NewLastPosition(),
		}
		g.lastposition.SetLastPosition(geo.NewPoint(42.1, 123))
		g.err.Set(errors.New("last position"))
		expectedPoint := geo.NewPoint(42.1, 123.)

		pos, alt, err := g.Position(context.Background(), nil)
		test.That(t, movementsensor.ArePointsEqual(point, expectedPoint), test.ShouldBeTrue)
		test.That(t, flt, test.ShouldEqual, 0.0)
		test.That(t, err, test.ShouldBeNil)
	})

}
