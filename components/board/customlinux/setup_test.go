//go:build linux

// Package customlinux implements a board running linux
package customlinux

import (
	"testing"

	"go.viam.com/test"
)

func TestConfigParse(t *testing.T) {
	emptyConfigPath := "./data/test_config_empty.json"
	_, err := parseBoardConfig(emptyConfigPath)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `"name" is required`)

	emptyPWMID := "./data/test_config_emptypwm.json"
	_, err = parseBoardConfig(emptyPWMID)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `must supply pwm_id for the pwm chip`)

	invalidRelativeID := "./data/test_config_invalid.json"
	_, err = parseBoardConfig(invalidRelativeID)
	test.That(t, err, test.ShouldNotBeNil)
	test.That(t, err.Error(), test.ShouldContainSubstring, `relative id on gpio chip must be less than ngpio`)
}
