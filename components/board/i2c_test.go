package board

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/utils"
)

func TestValidateI2C(t *testing.T) {
	fakecfg := &I2CAttrConfig{I2CBus: "some-bus"}

	path := "path"
	err := fakecfg.ValidateI2C(path, true)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "i2c_addr"))
	err = fakecfg.ValidateI2C(path, false)
	test.That(t, err, test.ShouldBeNil)

	fakecfg.I2cAddr = 66
	err = fakecfg.ValidateI2C(path, true)
	test.That(t, err, test.ShouldBeNil)
	fakecfg.I2CBus = ""
	err = fakecfg.ValidateI2C(path, true)
	test.That(t, err, test.ShouldBeError,
		utils.NewConfigValidationFieldRequiredError(path, "i2c_bus"))
}
