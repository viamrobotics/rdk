//go:build linux

// Package customlinux implements a board running linux
package customlinux

import (
	"testing"

	"go.viam.com/test"
	"go.viam.com/rdk/components/board"
)

func TestConfigParse(t *testing.T) {
	emptyConfigPath := "./data/test_config_empty.json"
	_, err := parsePinConfig(emptyConfigPath)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	emptyPWMID := "./data/test_config_emptypwm.json"
	_, err = parsePinConfig(emptyPWMID)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must supply pwm_id for the pwm chip")

	invalidRelativeID := "./data/test_config_invalid.json"
	_, err = parsePinConfig(invalidRelativeID)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "relative id on gpio chip must be less than ngpio")
}

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	_, err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file or directory")

	validConfig.PinConfigFilePath = "./data"
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "path.digital_interrupts.0")
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

}
