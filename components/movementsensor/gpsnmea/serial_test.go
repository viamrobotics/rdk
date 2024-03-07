package gpsnmea

import (
	"context"
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	rutils "go.viam.com/rdk/utils"
)

func TestValidateSerial(t *testing.T) {
	fakecfg := &SerialConfig{}
	path := "path"
	err := fakecfg.validateSerial(path)
	test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError(path, "serial_path"))

	fakecfg.SerialPath = "some-path"
	err = fakecfg.validateSerial(path)
	test.That(t, err, test.ShouldBeNil)
}

func TestNewSerialMovementSensor(t *testing.T) {
	var deps resource.Dependencies
	path := "somepath"

	cfig := resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		Attributes: rutils.AttributeMap{
			"path":            "",
			"correction_path": "",
		},
	}

	logger := logging.NewTestLogger(t)
	ctx := context.Background()

	g, err := newNMEAGPS(ctx, deps, cfig, logger)
	test.That(t, g, test.ShouldBeNil)
	test.That(t, err, test.ShouldNotBeNil)

	cfig = resource.Config{
		Name:  "movementsensor1",
		Model: resource.DefaultModelFamily.WithModel("gps-nmea"),
		API:   movementsensor.API,
		ConvertedAttributes: &Config{
			ConnectionType: "serial",
			SerialConfig: &SerialConfig{
				SerialPath:     path,
				SerialBaudRate: 0,
			},
		},
	}
	g, err = newNMEAGPS(ctx, deps, cfig, logger)
	passErr := "open " + path + ": no such file or directory"
	if err == nil || err.Error() != passErr {
		test.That(t, err, test.ShouldBeNil)
		test.That(t, g, test.ShouldNotBeNil)
	}
}

func TestCloseSerial(t *testing.T) {
	logger := logging.NewTestLogger(t)
	ctx := context.Background()
	cancelCtx, cancelFunc := context.WithCancel(ctx)
	g := &NMEAMovementSensor{
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
	}

	err := g.Close(ctx)
	test.That(t, err, test.ShouldBeNil)
}
