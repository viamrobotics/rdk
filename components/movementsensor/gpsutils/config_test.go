package gpsutils

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestValidateSerial(t *testing.T) {
	fakecfg := &SerialConfig{}
	path := "path"
	err := fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError, resource.NewConfigValidationFieldRequiredError(path, "serial_path"))

	fakecfg.SerialPath = "some-path"
	err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeNil)
}

func TestValidateI2C(t *testing.T) {
	fakecfg := &I2CConfig{I2CBus: "1"}

	path := "path"
	err := fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeError,
		resource.NewConfigValidationFieldRequiredError(path, "i2c_addr"))

	fakecfg.I2CAddr = 66
	err = fakecfg.Validate(path)
	test.That(t, err, test.ShouldBeNil)
}
