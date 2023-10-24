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
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
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
	DisableNMEA    bool   `json:"disable_nmea,omitempty"`

	*SerialConfig `json:"serial_attributes,omitempty"`
	*I2CConfig    `json:"i2c_attributes,omitempty"`
}

// SerialConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialConfig struct {
	SerialPath     string `json:"serial_path"`
	SerialBaudRate int    `json:"serial_baud_rate,omitempty"`

	// TestChan is a fake "serial" path for test use only
	TestChan chan []uint8 `json:"-"`
}

// I2CConfig is used for converting Serial NMEA MovementSensor config attributes.
type I2CConfig struct {
	Board       string `json:"board"`
	I2CBus      string `json:"i2c_bus"`
	I2CAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string

	if cfg.ConnectionType == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	switch strings.ToLower(cfg.ConnectionType) {
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CConfig.validateI2C(path)
	case serialStr:
		return nil, cfg.SerialConfig.validateSerial(path)
	default:
		return nil, connectionTypeError(cfg.ConnectionType, serialStr, i2cStr)
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CConfig) validateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2CAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}
	if cfg.Board == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *SerialConfig) validateSerial(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

var model = resource.DefaultModelFamily.WithModel("gps-nmea")

// NmeaMovementSensor implements a gps that sends nmea messages for movement data.
type NmeaMovementSensor interface {
	movementsensor.MovementSensor
	Start(ctx context.Context) error                 // Initialize and run MovementSensor
	Close(ctx context.Context) error                 // Close MovementSensor
	ReadFix(ctx context.Context) (int, error)        // Returns the fix quality of the current MovementSensor measurements
	ReadSatsInView(ctx context.Context) (int, error) // Returns the number of satellites in view
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
		return NewSerialGPSNMEA(ctx, conf.ResourceName(), newConf, logging.FromZapCompatible(logger))
	case i2cStr:
		return NewPmtkI2CGPSNMEA(ctx, deps, conf.ResourceName(), newConf, logging.FromZapCompatible(logger))
	default:
		return nil, connectionTypeError(
			newConf.ConnectionType,
			i2cStr,
			serialStr)
	}
}
