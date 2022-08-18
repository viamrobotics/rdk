package board

import (
	"testing"

	"go.viam.com/test"
)

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	validConfig.Analogs = []AnalogConfig{{}}
	err := validConfig.Validate("path")
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
	err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `path.digital_interrupts.0`)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"pin" is required`)

	validConfig.DigitalInterrupts = []DigitalInterruptConfig{{Name: "bar", Pin: "3"}}
	test.That(t, validConfig.Validate("path"), test.ShouldBeNil)
}
