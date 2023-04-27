// Package gpsnmea implements an NMEA serial gps.
package gpsnmea

import (
	"context"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.viam.com/utils"

	"go.viam.com/rdk/components/movementsensor"
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
	Board          string `json:"board,omitempty"`
	DisableNMEA    bool   `json:"disable_nmea,omitempty"`

	*SerialConfig `json:"serial_attributes,omitempty"`
	*I2CConfig    `json:"i2c_attributes,omitempty"`
}

// SerialConfig is used for converting Serial NMEA MovementSensor config attributes.
type SerialConfig struct {
	SerialPath               string `json:"serial_path"`
	SerialBaudRate           int    `json:"serial_baud_rate,omitempty"`
	SerialCorrectionPath     string `json:"serial_correction_path,omitempty"`
	SerialCorrectionBaudRate int    `json:"serial_correction_baud_rate,omitempty"`
}

// I2CConfig is used for converting Serial NMEA MovementSensor config attributes.
type I2CConfig struct {
	I2CBus      string `json:"i2c_bus"`
	I2cAddr     int    `json:"i2c_addr"`
	I2CBaudRate int    `json:"i2c_baud_rate,omitempty"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.Board == "" && cfg.ConnectionType == i2cStr {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
	}

	if cfg.ConnectionType == "" {
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}

	switch cfg.ConnectionType {
	case i2cStr:
		if cfg.Board == "" {
			return nil, utils.NewConfigValidationFieldRequiredError(path, "board")
		}
		deps = append(deps, cfg.Board)
		return deps, cfg.I2CConfig.ValidateI2C(path)
	case serialStr:
		return nil, cfg.SerialConfig.ValidateSerial(path)
	default:
		return nil, utils.NewConfigValidationFieldRequiredError(path, "connection_type")
	}
}

// ValidateI2C ensures all parts of the config are valid.
func (cfg *I2CConfig) ValidateI2C(path string) error {
	if cfg.I2CBus == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_bus")
	}
	if cfg.I2cAddr == 0 {
		return utils.NewConfigValidationFieldRequiredError(path, "i2c_addr")
	}

	return nil
}

// ValidateSerial ensures all parts of the config are valid.
func (cfg *SerialConfig) ValidateSerial(path string) error {
	if cfg.SerialPath == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "serial_path")
	}
	return nil
}

var model = resource.DefaultModelFamily.WithModel("gps-nmea")

// NmeaMovementSensor implements a gps that sends nmea messages for movement data.
type NmeaMovementSensor interface {
	movementsensor.MovementSensor
	Start(ctx context.Context) error          // Initialize and run MovementSensor
	Close(ctx context.Context) error          // Close MovementSensor
	ReadFix(ctx context.Context) (int, error) // Returns the fix quality of the current MovementSensor measurements
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
	i2cStr         = "I2C"
	serialStr      = "serial"
	rtkStr         = "rtk"
)

func newNMEAGPS(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	switch newConf.ConnectionType {
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
