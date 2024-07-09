//go:build linux

// Package gpsrtk implements a gps. This file is for testing the config validation for an I2C chip.
package gpsrtk

import (
	"testing"

	"go.viam.com/test"

	"go.viam.com/rdk/resource"
)

func TestValidatePMTKRTK(t *testing.T) {
	path := "path"
	validConfig := I2CConfig{
		I2CBus:  "1",
		I2CAddr: 66,

		NtripURL:             "http://fakeurl",
		NtripConnectAttempts: 10,
		NtripMountpoint:      "NYC",
		NtripPass:            "somepass",
		NtripUser:            "someuser",
	}
	t.Run("valid config", func(t *testing.T) {
		cfg := validConfig
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeNil)
	})

	t.Run("missing bus", func(t *testing.T) {
		cfg := validConfig
		cfg.I2CBus = ""
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "i2c_bus"))
	})

	t.Run("missing address", func(t *testing.T) {
		cfg := validConfig
		cfg.I2CAddr = 0
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "i2c_addr"))
	})

	t.Run("missing url", func(t *testing.T) {
		cfg := validConfig
		cfg.NtripURL = ""
		_, err := cfg.Validate(path)
		test.That(t, err, test.ShouldBeError,
			resource.NewConfigValidationFieldRequiredError(path, "ntrip_url"))
	})
}
