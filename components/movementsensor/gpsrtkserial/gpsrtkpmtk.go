//go:build linux

// Package gpsrtkpmtk implements a gps using serial connection
package gpsrtkserial

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd), over I2C bus.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Example configuration:

	{
		"name": "my-gps-rtk",
		"type": "movement_sensor",
		"model": "gps-nmea-rtk-pmtk",
		"attributes": {
			"i2c_bus": "1",
			"i2c_addr": 66,
			"i2c_baud_rate": 115200,
			"ntrip_connect_attempts": 12,
			"ntrip_mountpoint": "MNTPT",
			"ntrip_password": "pass",
			"ntrip_url": "http://ntrip/url",
			"ntrip_username": "usr"
		},
		"depends_on": [],
	}

*/

import (
	"context"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-pmtk")

// I2CConfig is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type I2CConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2CAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`

	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *I2CConfig) Validate(path string) ([]string, error) {
	err := cfg.validateI2C(path)
	if err != nil {
		return nil, err
	}

	err = cfg.validateNtrip(path)
	if err != nil {
		return nil, err
	}

	return []string{}, nil
}

// validateI2C ensures all parts of the config are valid.
func (cfg *I2CConfig) validateI2C(path string) error {
	if cfg.I2CBus == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return resource.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}
	return nil
}

// validateNtrip ensures all parts of the config are valid.
func (cfg *I2CConfig) validateNtrip(path string) error {
	if cfg.NtripURL == "" {
		return resource.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}
	return nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *I2CConfig]{
			Constructor: newRTKI2C,
		})
}

func newRTKI2C(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	return makeRTKI2C(ctx, deps, conf, logger, nil)
}

// makeRTKI2C is separate from newRTKI2C, above, so we can pass in a non-nil mock I2C bus during
// unit tests.
func makeRTKI2C(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
	mockI2c buses.I2C,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*I2CConfig](conf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	g := &rtkSerial{
		Named:        conf.ResourceName().AsNamed(),
		cancelCtx:    cancelCtx,
		cancelFunc:   cancelFunc,
		logger:       logger,
		err:          movementsensor.NewLastError(1, 1),
	}

	config := gpsutils.I2CConfig{
		I2CBus:      newConf.I2CBus,
		I2CBaudRate: newConf.I2CBaudRate,
		I2CAddr:     newConf.I2CAddr,
	}
	if config.I2CBaudRate == 0 {
		config.I2CBaudRate = 115200
	}

	// If we have a mock I2C bus, pass that in, too. If we don't, it'll be nil and constructing the
	// reader will create a real I2C bus instead.
	dev, err := gpsutils.NewI2cDataReader(config, mockI2c, logger)
	if err != nil {
		return nil, err
	}
	g.cachedData = gpsutils.NewCachedData(dev, logger)

	g.correctionWriter = NewCorrectionWriter(newConf.I2CBus, byte(newConf.I2CAddr))

	if err := g.start(); err != nil {
		return nil, err
	}
	return g, nil
}
