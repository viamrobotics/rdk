//go:build linux

// Package customlinux implements a board running linux
package customlinux

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func TestConfigParse(t *testing.T) {
	logger := logging.NewTestLogger(t)
	emptyConfig := []byte(`{"pins": [{}]}`)
	_, err := parseRawPinData(emptyConfig, "path", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, resource.GetFieldFromFieldRequiredError(err), test.ShouldEqual, "name")

	emptyPWMID := []byte(`{"pins": [{"name": "7", "device_name": "gpiochip1", "line_number": 71, "pwm_chip_sysfs_dir": "hi"}]}`)
	_, err = parseRawPinData(emptyPWMID, "path", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "must supply pwm_id for the pwm chip")

	invalidLineNumber := []byte(`{"pins": [{"name": "7", "device_name": "gpiochip1", "line_number": -2}]}`)
	_, err = parseRawPinData(invalidLineNumber, "path", logger)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, "line_number on gpio chip must be at least zero")

	validConfig := []byte(`{"pins": [{"name": "7", "device_name": "gpiochip1", "line_number": 80}]}`)
	data, err := parseRawPinData(validConfig, "path", logger)
	correctData := make([]genericlinux.PinDefinition, 1)
	correctData[0] = genericlinux.PinDefinition{
		Name:       "7",
		DeviceName: "gpiochip1",
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

	validConfig.BoardDefsFilePath = "./"
	_, err = validConfig.Validate("path")
	test.That(t, err, test.ShouldBeNil)
}
