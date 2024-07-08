//go:build linux

// Package gpsrtk implements a gps. This file is for connecting to the chip over I2C.
package gpsrtk

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.viam.com/rdk/components/board/genericlinux/buses"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var i2cRtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-pmtk")

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
		i2cRtkmodel,
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
	_ resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
	mockI2c buses.I2C,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*I2CConfig](conf)
	if err != nil {
		return nil, err
	}

	cancelCtx, cancelFunc := context.WithCancel(context.Background())
	g := &gpsrtk{
		Named:      conf.ResourceName().AsNamed(),
		cancelCtx:  cancelCtx,
		cancelFunc: cancelFunc,
		logger:     logger,
		err:        movementsensor.NewLastError(1, 1),
	}

	ntripConfig := &gpsutils.NtripConfig{
		NtripURL:             newConf.NtripURL,
		NtripUser:            newConf.NtripUser,
		NtripPass:            newConf.NtripPass,
		NtripMountpoint:      newConf.NtripMountpoint,
		NtripConnectAttempts: newConf.NtripConnectAttempts,
	}

	g.ntripClient, err = gpsutils.NewNtripInfo(ntripConfig, g.logger)
	if err != nil {
		return nil, err
	}
	g.InputProtocol = "i2c"

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

	g.correctionWriter, err = newI2CCorrectionWriter(newConf.I2CBus, byte(newConf.I2CAddr))
	if err != nil {
		return nil, err
	}

	if err := g.start(); err != nil {
		return nil, err
	}

	// It's possible that we've taken so long to start up that the resource manager has given up on
	// us and tried constructing a new component instead. If that happens, we don't want 2
	// components talking to the same chip. So, if our context is canceled, close our component
	// instead of returning it.
	if ctx.Err() != nil {
		logger.Warn("context canceled by the end of the constructor! Closing the new component...")
		return nil, fmt.Errorf("timed out constructing I2C RTK reader. Closing: %w", g.Close(ctx))
	}

	return g, nil
}

func newI2CCorrectionWriter(busname string, address byte) (io.ReadWriteCloser, error) {
	bus, err := buses.NewI2cBus(busname)
	if err != nil {
		return nil, err
	}
	handle, err := bus.OpenHandle(address)
	if err != nil {
		return nil, err
	}
	correctionWriter := i2cCorrectionWriter{
		bus:    bus,
		handle: handle,
	}
	return &correctionWriter, nil
}

// This implements the io.ReadWriteCloser interface.
type i2cCorrectionWriter struct {
	bus    buses.I2C
	handle buses.I2CHandle
}

func (i *i2cCorrectionWriter) Read(p []byte) (int, error) {
	return 0, errors.New("unimplemented")
}

func (i *i2cCorrectionWriter) Write(p []byte) (int, error) {
	err := i.handle.Write(context.Background(), p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (i *i2cCorrectionWriter) Close() error {
	return i.handle.Close()
}
