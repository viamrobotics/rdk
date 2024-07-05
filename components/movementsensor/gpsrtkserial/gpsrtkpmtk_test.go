//go:build linux

package gpsrtkserial

import (
	"context"
	"testing"

	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor/fake"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testRoverName   = "testRover"
	testStationName = "testStation"
	testI2cBus      = "1"
	testI2cAddr     = 44
)

func TestValidateRTK(t *testing.T) {
	path := "path"
	cfg := I2CConfig{
		NtripURL:             "http://fakeurl",
		NtripConnectAttempts: 10,
		NtripPass:            "somepass",
		NtripUser:            "someuser",
		NtripMountpoint:      "NYC",
		I2CBus:               testI2cBus,
		I2CAddr:              testI2cAddr,
	}
	t.Run("valid config", func(t *testing.T) {
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("invalid ntrip url", func(t *testing.T) {
		cfg := I2CConfig{
			NtripURL:             "",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			I2CBus:               testI2cBus,
			I2CAddr:              testI2cAddr,
		}
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "ntrip_url"))
	})

	t.Run("invalid i2c bus", func(t *testing.T) {
		cfg := I2CConfig{
			I2CBus:               "",
			NtripURL:             "http://fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			I2CAddr:              testI2cAddr,
		}
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "i2c_bus"))
	})

	t.Run("invalid i2c addr", func(t *testing.T) {
		cfg := I2CConfig{
			I2CAddr:              0,
			NtripURL:             "http://fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			I2CBus:               testI2cBus,
		}
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "i2c_addr"))
	})
}

func TestReconfigure(t *testing.T) {
	mockI2c := inject.I2C{}
	g := &rtkI2C{
		wbaud:   9600,
		addr:    byte(66),
		mockI2c: &mockI2c,
		logger:  logging.NewTestLogger(t),
	}
	conf := resource.Config{
		Name: "reconfig1",
		ConvertedAttributes: &I2CConfig{
			NtripURL:             "http://fakeurl",
			NtripConnectAttempts: 10,
			NtripPass:            "somepass",
			NtripUser:            "someuser",
			NtripMountpoint:      "NYC",
			I2CBus:               testI2cBus,
			I2CAddr:              testI2cAddr,
			I2CBaudRate:          115200,
		},
	}
	err := g.Reconfigure(context.Background(), nil, conf)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g.wbaud, test.ShouldEqual, 115200)
	test.That(t, g.addr, test.ShouldEqual, byte(44))
}

type CustomMovementSensor struct {
	*fake.MovementSensor
	PositionFunc func(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error)
}

func (c *CustomMovementSensor) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	if c.PositionFunc != nil {
		return c.PositionFunc(ctx, extra)
	}
	// Fallback to the default implementation if PositionFunc is not set.
	return c.MovementSensor.Position(ctx, extra)
}
