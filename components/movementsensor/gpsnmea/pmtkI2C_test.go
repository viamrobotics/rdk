//go:build linux

package gpsnmea

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/testutils/inject"
)

const (
	testBoardName = "board1"
	testBusName   = "1"
)

func createMockI2c() buses.I2C {
	i2c := &inject.I2C{}
	handle := &inject.I2CHandle{}
	handle.WriteFunc = func(ctx context.Context, b []byte) error {
		return nil
	}
	handle.ReadFunc = func(ctx context.Context, count int) ([]byte, error) {
		return nil, nil
	}
	handle.CloseFunc = func() error {
		return nil
	}
	i2c.OpenHandleFunc = func(addr byte) (buses.I2CHandle, error) {
		return handle, nil
	}
	return i2c
}

func TestNewI2CMovementSensor(t *testing.T) {
	conf := resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
	}

	var deps resource.Dependencies
	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	// We try constructing a "real" component here, expecting that we never get past the config
	// validation step.
	g1, err := newNMEAGPS(ctx, deps, conf, logger)
	test.That(t, g1, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	conf = resource.Config{
		Name:  "movementsensor2",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			ConnectionType: "I2C",
			I2CConfig:      &gpsutils.I2CConfig{I2CBus: testBusName},
		},
	}
	config, err := resource.NativeConfig[*Config](conf)
	test.That(t, err, test.ShouldBeNil)
	mockI2c := createMockI2c()

	// This time, we *do* expect to construct a real object, so we need to pass in a mock I2C bus.
	g2, err := MakePmtkI2cGpsNmea(ctx, deps, conf.ResourceName(), config, logger, mockI2c)
	test.That(t, err, test.ShouldBeNil)
	test.That(t, g2.Close(context.Background()), test.ShouldBeNil)
	test.That(t, g2, test.ShouldNotBeNil)
}
