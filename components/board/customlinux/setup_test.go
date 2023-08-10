//go:build linux

// Package customlinux implements a board running linux
package customlinux

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
)

func TestConfigParse(t *testing.T) {
	emptyConfig := []byte(`{"pins": [{}]}`)
	_, err := parseRawPinData(emptyConfig, "path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	emptyPWMID := []byte(`{"pins": [{"name": "7", "ngpio": 86, "line_number": 71, "pwm_chip_sysfs_dir": "hi"}]}`)
	_, err = parseRawPinData(emptyPWMID, "path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must supply pwm_id for the pwm chip")

	invalidLineNumber := []byte(`{"pins": [{"name": "7", "ngpio": 86, "line_number": 100}]}`)
	_, err = parseRawPinData(invalidLineNumber, "path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "line_number on gpio chip must be less than ngpio")

	validConfig := []byte(`{"pins": [{"name": "7", "ngpio": 86, "line_number": 80}]}`)
	data, err := parseRawPinData(validConfig, "path")
	correctData := make([]genericlinux.PinDefinition, 1)
	correctData[0] = genericlinux.PinDefinition{
		Name:       "7",
		Ngpio:      86,
		LineNumber: 80,
		PwmID:      -1,
	}
	test.That(t, err, test.ShouldBeNil)
	test.That(t, data, test.ShouldResemble, correctData)
}

func TestConfigValidate(t *testing.T) {
	validConfig := Config{}

	_, err := validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "no such file or directory")

	validConfig.PinConfigFilePath = "./"
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "path.digital_interrupts.0")
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	validConfig.DigitalInterrupts = []board.DigitalInterruptConfig{{Name: "20"}}
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "path.digital_interrupts.0")
	test.That(t, err.Error(), test.ShouldContainSubstring, `"pin" is required`)
}
