// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

/*
	This package supports GPS NMEA over Serial or I2C.

	NMEA reference manual:
	https://www.sparkfun.com/datasheets/GPS/NMEA%20Reference%20Manual-Rev2.1-Dec07.pdf

	Example GPS NMEA chip datasheet:
	https://content.u-blox.com/sites/default/files/NEO-M9N-00B_DataSheet_UBX-19014285.pdf

*/

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/components/movementsensor/gpsutils"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

func connectionTypeError(connType, serialConn, i2cConn string) error {
	return errors.Errorf("%s is not a valid connection_type of %s, %s",
		connType,
		serialConn,
		i2cConn)
}

// Config is used for converting NMEA Movement Sensor attibutes.
type Config struct {
	ConnectionType string `json:"connection_type"`

	*gpsutils.SerialConfig `json:"serial_attributes,omitempty"`
	*gpsutils.I2CConfig    `json:"i2c_attributes,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	if cfg.ConnectionType == "" {
		return nil, resource.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	switch strings.ToLower(cfg.ConnectionType) {
	case i2cStr:
		return nil, cfg.I2CConfig.Validate(path)
	case serialStr:
		return nil, cfg.SerialConfig.Validate(path)
	default:
		return nil, connectionTypeError(cfg.ConnectionType, serialStr, i2cStr)
	}
}

var model = resource.DefaultModelFamily.WithModel("gps-nmea")

// NmeaMovementSensor implements a gps that sends nmea messages for movement data.
type NmeaMovementSensor interface {
	movementsensor.MovementSensor
	Close(ctx context.Context) error // Close MovementSensor
}

func init() {
	resource.RegisterComponent(
		movementsensor.API,
		model,
		resource.Registration[movementsensor.MovementSensor, *Config]{
			Constructor: newNMEAGPS,
		})
}

const (
	connectionType = "connection_type"
	i2cStr         = "i2c"
	serialStr      = "serial"
	rtkStr         = "rtk"
)

func newNMEAGPS(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(newConf.ConnectionType) {
	case serialStr:
		return NewSerialGPSNMEA(ctx, conf.ResourceName(), newConf, logger)
	case i2cStr:
		return NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), newConf, logger)
	default:
		return nil, connectionTypeError(
			newConf.ConnectionType,
			i2cStr,
			serialStr)
	}
}
