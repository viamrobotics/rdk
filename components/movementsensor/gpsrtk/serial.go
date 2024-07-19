// Package gpsrtk implements a gps
package gpsrtk

/*
	This package supports GPS RTK (Real Time Kinematics), which takes in the normal signals
	from the GNSS (Global Navigation Satellite Systems) along with a correction stream to achieve
	positional accuracy (accuracy tbd). This file is for the implementation that connects over a
	serial port.

	Example GPS RTK chip datasheet:
	https://content.u-blox.com/sites/default/files/ZED-F9P-04B_DataSheet_UBX-21044850.pdf

	Ntrip Documentation:
	https://gssc.esa.int/wp-content/uploads/2018/07/NtripDocumentation.pdf

	Example configuration:
	{
      "type": "movement_sensor",
	  "model": "gps-nmea-rtk-serial",
      "name": "my-gps-rtk"
      "attributes": {
        "ntrip_url": "url",
        "ntrip_username": "usr",
        "ntrip_connect_attempts": 10,
        "ntrip_mountpoint": "MTPT",
        "ntrip_password": "pwd",
		"serial_baud_rate": 115200,
        "serial_path": "serial-path"
      },
      "depends_on": [],
    }

*/

import (
	"context"
	"fmt"
	"io"

	slib "github.com/jacobsa/go-serial/serial"
	"go.uber.org/multierr"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var rtkmodel = resource.DefaultModelFamily.WithModel("gps-nmea-rtk-serial")

// SerialConfig is used for converting NMEA MovementSensor with RTK capabilities config attributes.
type SerialConfig struct {
	SerialPath     string `json:"serial_path"`
	SerialBaudRate int    `json:"serial_baud_rate,omitempty"`

	NtripURL             string `json:"ntrip_url"`
	NtripConnectAttempts int    `json:"ntrip_connect_attempts,omitempty"`
	NtripMountpoint      string `json:"ntrip_mountpoint,omitempty"`
	NtripPass            string `json:"ntrip_password,omitempty"`
	NtripUser            string `json:"ntrip_username,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *SerialConfig) Validate(path string) ([]string, error) {
	if cfg.SerialPath == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "serial_path")
	}

	if cfg.NtripURL == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "ntrip_url")
	}

	return nil, nil
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		rtkmodel,
		resource.Registration[movementsensor.MovementSensor, *SerialConfig]{
			Constructor: newRTKSerial,
		})
}

func newRTKSerial(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*SerialConfig](conf)
	if err != nil {
		return nil, err
	}

	g := &gpsrtk{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
		err:    movementsensor.NewLastError(1, 1),
	}

	if newConf.SerialPath != "" {
		g.writePath = newConf.SerialPath
		g.logger.CInfof(ctx, "updated serial_path to %#v", newConf.SerialPath)
	}

	if newConf.SerialBaudRate != 0 {
		g.wbaud = newConf.SerialBaudRate
		g.logger.CInfof(ctx, "updated serial_baud_rate to %v", newConf.SerialBaudRate)
	} else {
		g.wbaud = 38400
		g.logger.CInfo(ctx, "serial_baud_rate using default baud rate 38400")
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

	serialConfig := &gpsutils.SerialConfig{
		SerialPath:     newConf.SerialPath,
		SerialBaudRate: newConf.SerialBaudRate,
	}
	dev, err := gpsutils.NewSerialDataReader(serialConfig, logger)
	if err != nil {
		return nil, err
	}
	g.cachedData = gpsutils.NewCachedData(dev, logger)

	// Initialize g.correctionWriter
	g.correctionWriter, err = newSerialCorrectionWriter(
		newConf.SerialPath, uint(newConf.SerialBaudRate))
	if err != nil {
		return nil, multierr.Combine(err, g.Close(ctx))
	}

	if err := g.start(); err != nil {
		return nil, multierr.Combine(err, g.Close(ctx))
	}

	return g, nil
}

// newSerialCorrectionWriter opens the serial port for writing.
func newSerialCorrectionWriter(filePath string, baud uint) (io.ReadWriteCloser, error) {
	options := slib.OpenOptions{
		PortName:        filePath,
		BaudRate:        baud,
		DataBits:        8,
		StopBits:        1,
		MinimumReadSize: 1,
	}

	var err error
	correctionWriter, err := slib.Open(options)
	if err != nil {
		return nil, fmt.Errorf("serial.Open: %w", err)
	}

	return correctionWriter, nil
}
