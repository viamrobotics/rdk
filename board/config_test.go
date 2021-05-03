package board

import (
	"testing"

	"github.com/edaniels/test"
)

func TestConfigValidate(t *testing.T) {
	var emptyConfig Config
	err := emptyConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig := Config{
		Name: "foo",
	}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Motors = []MotorConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.motors.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Motors = []MotorConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Servos = []ServoConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.servos.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Servos = []ServoConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.Analogs = []AnalogConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.analogs.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.Analogs = []AnalogConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)

	validConfig.DigitalInterrupts = []DigitalInterruptConfig{{}}
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.DigitalInterrupts = []DigitalInterruptConfig{{Name: "bar"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}
